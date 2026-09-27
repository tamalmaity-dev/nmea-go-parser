// Command nmea-dump is a diagnostic tool: it reads NMEA from a serial port, a
// file, or standard input, and prints what the library decoded.
//
// It is the fastest way to see what a receiver is actually sending, which is
// usually the first step when a position looks wrong.
//
//	go run ./cmd/nmea-dump -port COM3 -baud 9600
//	go run ./cmd/nmea-dump -file testdata/sample.nmea
//	go run ./cmd/nmea-dump -raw -file testdata/sample.nmea
//	go run ./cmd/nmea-dump -list
//	go run ./cmd/nmea-dump -formatters
//	go run ./cmd/nmea-dump -version
//	cat capture.log | go run ./cmd/nmea-dump -file -
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/device"
	"github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

// These are set by the linker, not by hand. The Makefile and the release
// workflow pass them in -X; a plain `go build` leaves them empty and
// devVersion stands in, so a binary that was never built through the Makefile
// says so rather than claiming a release number.
var (
	version = ""
	commit  = ""
	date    = ""
)

// devVersion is what an unlinked build reports. It is deliberately not a
// version number: a support request that quotes "0.0.0" needs to be
// distinguishable from one that quotes a real release.
const devVersion = "dev"

func buildVersion() string {
	if version == "" {
		return devVersion
	}
	return version
}

type options struct {
	port        string
	baud        int
	file        string
	raw         bool
	showVersion bool
	showBuild   bool
	listPorts   bool
	formatters  bool
	sky         bool
	once        bool
}

func main() {
	var o options
	flag.StringVar(&o.port, "port", "", "serial port, e.g. COM3 or /dev/ttyUSB0")
	flag.IntVar(&o.baud, "baud", 9600, "baud rate")
	flag.StringVar(&o.file, "file", "", `read from a file instead of a port, or "-" for stdin`)
	flag.BoolVar(&o.raw, "raw", false, "print every sentence, not just position changes")
	flag.BoolVar(&o.showVersion, "receiver", false, "report the receiver and standard version, then exit")
	flag.BoolVar(&o.showBuild, "version", false, "print this tool's version and exit")
	flag.BoolVar(&o.listPorts, "list", false, "list available serial ports and exit")
	flag.BoolVar(&o.formatters, "formatters", false, "list the sentence formatters this build decodes")
	flag.BoolVar(&o.sky, "sky", false, "print the satellite sky: per-system counts, then every tracked satellite")
	flag.BoolVar(&o.once, "once", false, "stop after the first fix (useful for scripting)")
	flag.Parse()

	switch {
	case o.showBuild:
		printBuild()
		return
	case o.listPorts:
		showPorts()
		return
	case o.formatters:
		showFormatters()
		return
	}

	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// printBuild reports what this binary is, so a bug report can be tied to an
// exact build. The commit and date are omitted when the linker did not supply
// them rather than printed as empty.
func printBuild() {
	fmt.Printf("%s %s\n", binaryName, buildVersion())
	if commit != "" {
		fmt.Printf("commit:     %s\n", commit)
	}
	if date != "" {
		fmt.Printf("built:      %s\n", date)
	}
	fmt.Printf("go:         %s\n", runtime.Version())
	fmt.Printf("platform:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("nmea:       up to %s\n", nmea.LatestVersion)
	fmt.Printf("formatters: %d\n", len(sentences.Formatters()))
}

// binaryName is the name this tool is distributed under, without the
// platform extension the release build appends.
const binaryName = "nmea-dump"

func run(o options) error {
	source, closeSource, err := openSource(o)
	if err != nil {
		return err
	}
	defer closeSource()

	parser := nmea.New()

	// Errors go to stderr so they can be filtered out of a data capture, and
	// are capped so a wrong baud rate does not produce a million lines.
	var errCount int
	parser.OnError(func(err error) {
		errCount++
		switch {
		case o.raw || errCount <= 10:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		case errCount == 11:
			fmt.Fprintln(os.Stderr, "further errors suppressed")
		}
	})

	// Positions only change when the receiver moves, so a fix report that
	// repeats the previous one is suppressed. At 10 Hz an unfiltered log is
	// unreadable, and the interesting moments are the changes.
	//
	// A handler cannot stop the stream by returning an error: the parser
	// reports it to the error handlers and keeps reading, which is the right
	// default for a data source. Cancelling the context is how a handler
	// asks for a clean shutdown, and that is what -once does.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	last := ""
	parser.OnFix(func(f *nmea.Fix) error {
		line := describeFix(f)
		if !o.raw && line == last {
			return nil
		}
		last = line
		fmt.Println(line)
		if o.once && f.HasPosition {
			stop()
		}
		return nil
	})

	if o.raw {
		parser.OnEvent(func(ev nmea.Event) error {
			status := "ok"
			if ev.Err != nil {
				status = "ERR"
			}
			fmt.Printf("%-4s %-10s %s\n", status, ev.Sentence.Address(), ev.Sentence.Raw)
			return nil
		})
	}

	// A GSV cycle spans several sentences, one per four satellites, so the
	// sky is printed when the last of them arrives rather than on every
	// sentence. Printing per sentence would show a partial sky most of the
	// time, which is the one thing a sky plot must not do.
	if o.sky {
		sentences.Watch(parser, "GSV", func(g sentences.GSV) error {
			if g.CycleComplete() {
				printSky(parser.Fix())
			}
			return nil
		})
	}

	// The receiver and standard version are inferred from traffic, so they
	// are only worth reporting once some has been seen.
	if o.showVersion {
		go func() {
			deadline := time.After(3 * time.Second)
			tick := time.NewTicker(100 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-deadline:
					reportVersion(parser)
					stop()
					return
				case <-ctx.Done():
					reportVersion(parser)
					return
				case <-tick.C:
					if v, reported := parser.Version(); reported && v.Major >= 4 {
						reportVersion(parser)
						stop()
						return
					}
				}
			}
		}()
	}

	err = parser.Consume(ctx, source)
	if err != nil && ctx.Err() == nil {
		return err
	}

	stats := parser.Stats()
	fmt.Fprintf(os.Stderr, "\n%v lines, %v bytes, %v sentences contributing to the fix, "+
		"%v bad checksums, %v unknown formatters, %v events dropped\n",
		stats.Lines, stats.Bytes, stats.Sentences, stats.BadChecksums, stats.Unknown, stats.Dropped)

	if o.showVersion {
		reportVersion(parser)
	}
	return nil
}

// openSource returns the reader to consume, along with a closer.
//
// A file of "-" means standard input, which is what makes a pipeline work:
//
//	cat capture.log | nmea-dump
func openSource(o options) (io.Reader, func(), error) {
	if o.file == "-" {
		return os.Stdin, func() {}, nil
	}
	if o.file != "" {
		f, err := os.Open(o.file)
		if err != nil {
			return nil, func() {}, err
		}
		return f, func() { f.Close() }, nil
	}

	name := o.port
	if name == "" {
		ports, err := device.Ports()
		if err != nil {
			return nil, func() {}, err
		}
		if len(ports) == 0 {
			return nil, func() {}, errors.New("no serial ports found; pass -port or -file")
		}
		if len(ports) > 1 {
			fmt.Fprintf(os.Stderr, "several ports present, using %s; pass -port to choose\n", ports[0])
		}
		name = ports[0]
	}

	// The read timeout makes a silent receiver detectable: without it a
	// disconnected device looks identical to one that is merely idle.
	dev, err := device.Open(device.Config{
		Port:        name,
		BaudRate:    o.baud,
		ReadTimeout: 2 * time.Second,
	})
	if err != nil {
		return nil, func() {}, err
	}
	fmt.Fprintf(os.Stderr, "reading %s at %d baud\n", name, o.baud)
	return dev, func() { dev.Close() }, nil
}

func reportVersion(p *nmea.Parser) {
	info := p.Receiver()
	if info.Manufacturer != "" || info.Product != "" || info.Software != "" {
		fmt.Fprintf(os.Stderr, "receiver: %s %s (software %s)\n",
			info.Manufacturer, info.Product, info.Software)
	}
	v, reported := p.Version()
	src := "inferred from the traffic"
	if reported {
		src = "stated by the receiver"
	}
	fmt.Fprintf(os.Stderr, "standard: NMEA %s (%s)\n", v, src)
}

// describeFix renders the fix as one line of the most useful fields.
func describeFix(f *nmea.Fix) string {
	if !f.HasPosition {
		return "no position yet"
	}

	var b strings.Builder
	if f.Valid {
		b.WriteString(f.Quality.String())
	} else {
		b.WriteString("NO FIX")
	}
	if f.HasNavStatus {
		fmt.Fprintf(&b, "/%s", f.NavStatus)
	}
	fmt.Fprintf(&b, "  %9.6f, %10.6f", f.Latitude, f.Longitude)

	if acc, ok := f.HorizontalAccuracy(); ok {
		fmt.Fprintf(&b, " +/-%.1fm", acc)
	}
	if f.HasSatellitesUsed || f.HasSatellitesView {
		fmt.Fprintf(&b, " %d/%d sats", f.SatellitesUsed, f.SatellitesInView)
	}
	if f.HasAltitude {
		fmt.Fprintf(&b, " %.1fm", f.Altitude)
	}
	if f.HasSpeed {
		fmt.Fprintf(&b, " %.1fkn", f.SpeedKnots)
	}
	if f.HasCourse {
		fmt.Fprintf(&b, " %03.0fdeg", f.CourseDegrees)
	}
	if f.HasHeading {
		ref := "mag"
		if f.HeadingTrue {
			ref = "true"
		}
		fmt.Fprintf(&b, " hdg%.0f%s", f.HeadingDegrees, ref)
	}
	if f.HasWaterSpeed {
		fmt.Fprintf(&b, " water%.1fkn", f.WaterSpeedKnots)
	}
	if f.HasWindSpeed {
		fmt.Fprintf(&b, " wind%.1fkn", f.WindSpeedKnots)
	}
	if f.HasRateOfTurn {
		fmt.Fprintf(&b, " rot%.1f", f.RateOfTurn)
	}
	if ts, ok := f.Timestamp(); ok {
		fmt.Fprintf(&b, " %s", ts.Format("2006-01-02 15:04:05"))
	} else if f.TimeOfDay.Available {
		fmt.Fprintf(&b, " %s", f.TimeOfDay)
	}
	if f.HasSatelliteFault {
		fmt.Fprintf(&b, " [sat%d suspect]", f.SatelliteFault)
	}
	return b.String()
}

// printSky renders the satellite picture: how many satellites each system
// contributes, then every tracked satellite with its band and whether it is in
// the solution.
//
// The per-system table comes first because it is the answer to the question a
// sky view is usually opened to ask, which is not "which satellite is that" but
// "which constellations am I actually using".
func printSky(f nmea.Fix) {
	bySystem := f.SatellitesByConstellation()
	if len(bySystem) == 0 {
		fmt.Println("no satellites reported yet")
		return
	}

	// The header states the total from the receiver, which is a different
	// measurement from the sum of the per-system counts: the receiver counts
	// everything it can hear, and a GSV cycle may not have been fully
	// received. Showing both makes that visible instead of hiding it.
	fmt.Printf("sky: %d tracked, %d used", len(f.Satellites), f.SatellitesUsed)
	if f.HasSatellitesView && f.SatellitesInView != len(f.Satellites) {
		fmt.Printf(" (receiver reports %d in view)", f.SatellitesInView)
	}
	fmt.Println()

	fmt.Printf("  %-8s %8s %6s  %s\n", "system", "tracked", "used", "bands")
	for _, cs := range bySystem {
		name := cs.Constellation.String()
		if !cs.ConstellationKnown {
			name += "?"
		}
		fmt.Printf("  %-8s %8d %6d  %s\n", name, cs.Tracked, cs.Used, bandsOf(f, cs.Constellation))
	}

	fmt.Println()
	fmt.Printf("  %-4s %4s  %-8s %5s %6s %6s\n", "sys", "prn", "band", "elev", "azim", "snr")
	for _, s := range f.SatelliteStatus() {
		mark := " "
		if s.Used {
			mark = "*"
		}
		band := "-"
		if name, ok := s.SignalName(); ok {
			band = name
		}
		snr := "-"
		if s.HasSNR {
			snr = fmt.Sprintf("%.0fdB", s.SNR)
		}
		sys := "?"
		if s.ConstellationKnown {
			sys = nmea.ConstellationAbbrev(s.Constellation)
		}
		fmt.Printf("%s %-4s %4d  %-8s %4.0f° %5.0f° %6s\n",
			mark, sys, s.ID, band, s.Elevation, s.Azimuth, snr)
	}
	fmt.Println("  (* = used in the solution)")
}

// bandsOf lists the distinct band names seen for one system, which is the
// quickest way to tell a single-band receiver from a multi-frequency one.
func bandsOf(f nmea.Fix, c nmea.Constellation) string {
	seen := make(map[string]bool)
	var order []string
	for _, s := range f.SatelliteStatus() {
		if s.Constellation != c {
			continue
		}
		name, ok := s.SignalName()
		if !ok {
			continue
		}
		if !seen[name] {
			seen[name] = true
			order = append(order, name)
		}
	}
	if len(order) == 0 {
		return "not reported"
	}
	sort.Strings(order)
	return strings.Join(order, " ")
}

// showPorts prints the serial ports the system can see.
func showPorts() {
	ports, err := device.Ports()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(ports) == 0 {
		fmt.Println("no serial ports found")
		return
	}
	for _, p := range ports {
		fmt.Println(p)
	}
}

func showFormatters() {
	got := sentences.Formatters()
	sort.Strings(got)
	for _, f := range got {
		fmt.Println(f)
	}
	fmt.Printf("\n%d formatters\n", len(got))
}
