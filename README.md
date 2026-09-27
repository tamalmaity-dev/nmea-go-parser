# nmea-go-parser

[![CI](https://github.com/tamalmaity-dev/nmea-go-parser/actions/workflows/ci.yml/badge.svg)](https://github.com/tamalmaity-dev/nmea-go-parser/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/tamalmaity-dev/nmea-go-parser.svg)](https://pkg.go.dev/github.com/tamalmaity-dev/nmea-go-parser)
[![Go Report Card](https://goreportcard.com/badge/github.com/tamalmaity-dev/nmea-go-parser)](https://goreportcard.com/report/github.com/tamalmaity-dev/nmea-go-parser)


This is a NMEA 0183 parser for the Go programming language (Golang). Point it at a
serial port, a TCP socket, a log file, or a pipe, and it hands you latitude,
longitude, altitude, speed, course, heading, satellite status, and fix quality
as typed Go values rather than strings you split yourself.

- **56 sentence formatters**, NMEA **2.00 through 4.30**
- **Serial, TCP, file, and stdin** input, plus a ready-made `nmea-dump` CLI
- **GPS, GLONASS, Galileo, BeiDou, QZSS, NavIC, SBAS** satellite tracking

## Contents

- [nmea-go-parser](#nmea-go-parser)
	- [Contents](#contents)
	- [Features](#features)
	- [Installing](#installing)
		- [As a library](#as-a-library)
		- [Staying up to date](#staying-up-to-date)
		- [As a command line tool](#as-a-command-line-tool)
		- [From source, with the Makefile](#from-source-with-the-makefile)
	- [Supported NMEA sentences](#supported-nmea-sentences)
	- [Examples](#examples)
		- [Parse one sentence](#parse-one-sentence)
		- [NMEA 4.10 tag blocks](#nmea-410-tag-blocks)
		- [Custom sentence parser](#custom-sentence-parser)
		- [Read a receiver](#read-a-receiver)
		- [File, socket, or any reader](#file-socket-or-any-reader)
		- [One sentence type](#one-sentence-type)
		- [The aggregate fix](#the-aggregate-fix)
		- [Command line tool](#command-line-tool)
		- [Command line output](#command-line-output)
			- [Normal](#normal)
			- [Raw](#raw)
			- [Satellite sky](#satellite-sky)
			- [Stream summary](#stream-summary)
	- [Packages](#packages)
	- [Documentation](#documentation)
	- [Contributing](#contributing)
	- [License](#license)

## Features

- **Parse individual NMEA 0183 sentences.** One line in, one typed Go value
  out, with no parser to set up.
- **Multiple input sources.** Serial ports, TCP connections, files, standard
  input, or any `io.Reader` you already have. The core never pulls in a serial
  dependency.
- **Typed values, not string splitting.** Every field is decoded into a Go
  struct with named fields, presence flags for optional values, and unit letters
  validated rather than ignored.
- **An aggregate `Fix`.** `GGA`, `RMC`, `GNS`, `GSV`, `GSA` and the rest merge
  into one current-state value you can read at any moment without tracking
  sentence order yourself.
- **Three output styles.** Callbacks for the merged fix, typed handlers for one
  sentence type, and a channel of every decoded event.
- **Multi-constellation satellites.** Per-system GSV and GSA merging, with the
  used-versus-tracked join done per constellation rather than per satellite
  number, and NMEA 4.10 signal IDs resolved against the right band table.
- **Support for sentences with NMEA 4.10 "TAG Blocks".** `!`-framed sentences,
  six-bit ASCII armouring, and tag blocks verified against their own separate
  checksum.
- **Register custom parsers for unsupported sentence types.** One file and one
  registration line, with no changes to the parser.
- **Checksums you control.** Calculate, verify, and generate them; keep parsing on
  a bad checksum by default or reject it with a strict option.
- **Survives bad input.** A corrupt line is reported through error handlers and
  skipped; the stream keeps reading. `Stats` counts lines, bytes, bad checksums,
  unknown formatters, and dropped events.
- **Safe to read while it runs.** `Fix()` and `Stats()` may be called from any
  goroutine. Register handlers and decoders before the stream starts — doing it
  while a read loop is running is a data race.
- **A CLI that shows you the wire.** `nmea-dump` reads a port, a file, or a pipe
  and prints normal, raw, or satellite-sky output.

## Installing

### As a library

To install nmea-go-parser use `go get`:

```sh
go get github.com/tamalmaity-dev/nmea-go-parser
```

This will then make the `github.com/tamalmaity-dev/nmea-go-parser` package available to you.

The core package depends only on the standard library. The serial
`device` package pulls in `go.bug.st/serial`, and is the only part of the
module that does.

Verify it resolved:

```sh
go list -m github.com/tamalmaity-dev/nmea-go-parser
```

### Staying up to date

To update nmea-go-parser to the latest version, use
`go get -u github.com/tamalmaity-dev/nmea-go-parser`.

### As a command line tool

Install `nmea-dump` into your `GOBIN`:

```sh
go install github.com/tamalmaity-dev/nmea-go-parser/cmd/nmea-dump@latest
```

Or build it from a local checkout:

```sh
git clone https://github.com/tamalmaity-dev/nmea-go-parser
cd nmea-go-parser
go build -o nmea-dump ./cmd/nmea-dump
```

Make sure your Go binary directory is on `PATH`, then confirm it works:

```sh
nmea-dump -version
```

```text
nmea-dump 0.1.4
go:         go1.26.3
platform:   windows/amd64
nmea:       up to 4.30
formatters: 56
```

If you used `go build` rather than the `Makefile`, the version reports as
`dev` because no version was passed to the linker. That is expected: a build
that never went through `make` or the release workflow says so instead of
claiming a release number.

### From source, with the Makefile

```sh
make help          # list every target
make build         # ./nmea-dump with the version compiled in
make test          # go test ./...
make check         # what CI runs: fmt, vet, test
make release       # cross-compile into ./dist with SHA256SUMS.txt
```

The Makefile uses `sh`, so on Windows run it from Git Bash or WSL. Every
target is a plain `go` command if you would rather not.

## Supported NMEA sentences

56 formatters, each in its own file, each documented field by field in
[docs/nmea.md](docs/nmea.md) against the standard, and each with a test that
checks every field against the published example sentence.

| Sentence | Carries |
|---|---|
| **Position and fix** | |
| `GGA` | GPS fix data: time, latitude, longitude, fix quality, satellites, HDOP, altitude, geoid separation |
| `GNS` | GNSS fix data: multi-constellation fix, NMEA 4.10 navigational status |
| `RMC` | Recommended minimum: time, position, speed, course, date, magnetic variation, mode |
| `GLL` | Geographic position: latitude, longitude, UTC, status |
| `VTG` | Course over ground and ground speed, true and magnetic, knots and km/h |
| **Satellites and integrity** | |
| `GSA` | GNSS DOP and active satellites: fix type, satellites in use, PDOP/HDOP/VDOP |
| `GSV` | Satellites in view: PRN, elevation, azimuth, signal strength, across a multi-sentence cycle |
| `GST` | Pseudorange noise statistics: RMS values, standard deviations of latitude, longitude, altitude, time |
| `GBS` | Satellite fault detection (RAIM): expected error and the satellite suspected |
| `GRS` | Range residuals: one residual per satellite in the fix |
| `ZDA` | Time and date: UTC, day, month, year, local zone offset |
| **Heading** | |
| `HDT` | Heading, true |
| `THS` | True heading with status: automatic or manual |
| `HDG` | Heading, deviation, and magnetic variation |
| `HDM` | Heading, magnetic |
| `HSC` | Heading steering command: the autopilot's commanded heading |
| **Speed, distance, and rate** | |
| `VHW` | Water speed and heading, true and magnetic |
| `VBW` | Dual ground and water speed |
| `VLW` | Distance travelled through water: log readings and total |
| `ROT` | Rate of turn, degrees per minute |
| `RPM` | Engine or shaft revolutions, speed, and pitch |
| **Navigation and cross-track** | |
| `WPL` | Waypoint location: latitude, longitude, id |
| `RTE` | Routes: message numbering and the waypoint list |
| `RMB` | Recommended minimum navigation information: bearing, distance, and speed to the destination |
| `APB` | Autopilot sentence B: bearing to waypoint, cross-track error, arrival status |
| `AAM` | Waypoint arrival alarm: whether the arrival radius has been reached |
| `BOD` | Bearing, origin to destination, true and magnetic |
| `XTE` | Cross-track error: magnitude, direction to steer, units |
| `BWC` | Bearing and distance to waypoint, great circle |
| `BWR` | Bearing and distance to waypoint, rhumb line |
| `WCV` | Waypoint closure velocity |
| **Environment and depth** | |
| `DPT` | Depth of water: depth, transducer offset, range scale |
| `DBT` | Depth below transducer, in feet, metres, and fathoms |
| `DBS` | Depth below the surface |
| `DBK` | Depth below the keel |
| `MTW` | Mean temperature of water |
| `MDA` | Meteorological composite: pressure, air and water temperature, humidity, dew point, wind |
| `MTA` | Air temperature |
| `MWD` | Wind direction and speed, true and magnetic |
| `MWV` | Wind speed and angle, relative or absolute |
| `DTM` | Datum reference: which geodetic datum the position uses |
| `XDR` | Transducer measurements: temperature, pressure, angle, and more, repeated per reading |
| **Wind** | |
| `VWR` | Relative wind: angle off the bow, port or starboard, speed in three units |
| `VWT` | True wind: angle and speed as corrected by the receiver |
| **Vessel** | |
| `OSD` | Own ship data: heading, rudder position, speed, trim, and heel |
| `RSA` | Rudder sensor angle |
| **Tracked targets** | |
| `TTM` | Tracked target message: range, bearing, speed, course, closest point of approach |
| `TLL` | Target latitude and longitude |
| `TLB` | Target label: a name for each tracked target |
| **AIS and alerting** | |
| `VDM` | AIS received message: fragment count, sequence id, channel, six-bit-armoured payload |
| `VDO` | AIS own-ship message, framed the same way |
| `ABM` | AIS addressed binary message, with the destination MMSI |
| `BBM` | AIS broadcast binary message |
| `ACK` | Alert acknowledgement |
| **Text and identification** | |
| `TXT` | Text transmission: free-form receiver messages |
| `VER` | Version and model identification: manufacturer, product, software version |

Two things worth stating plainly about the AIS rows. They decode the
**envelope**: fragment numbering, sequence id, channel, and the payload as
bytes. The payload's *meaning* is ITU-R M.1371, a separate protocol, and is
deliberately not decoded here. And the four proprietary sample lines
(`PUBX`, `PMTK001`, `PGRME`, `PMTKVER`) are receiver extensions with no NMEA
formatter, so they are reported as unknown rather than guessed at.

## Examples

### Parse one sentence

```go
package main

import (
	"fmt"
	"log"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

func main() {
	sentence := "$GPRMC,220516,A,5133.82,N,00042.24,W,173.8,231.8,130694,004.2,W*70"
	s, err := nmea.ParseSentence(sentence)
	if err != nil {
		log.Fatal(err)
	}
	v, err := nmea.DefaultRegistry.Decode(s)
	if err != nil {
		log.Fatal(err)
	}
	if nmea.DataTypeOf(v) == nmea.TypeRMC {
		m := v.(sentences.RMC)
		fmt.Printf("Raw sentence:  %s\n", m.Raw())
		fmt.Printf("Checksum OK:   %v\n", s.ChecksumOK())
		fmt.Printf("Talker:        %s\n", m.Talker())
		fmt.Printf("Time:          %s\n", m.UTC)
		fmt.Printf("Status:        %s\n", m.Status)
		fmt.Printf("Latitude GPS:  %s\n", nmea.FormatGPS(m.Latitude))
		fmt.Printf("Latitude DMS:  %s\n", nmea.FormatDMS(m.Latitude))
		fmt.Printf("Longitude GPS: %s\n", nmea.FormatGPS(m.Longitude))
		fmt.Printf("Longitude DMS: %s\n", nmea.FormatDMS(m.Longitude))
		fmt.Printf("Speed:         %f\n", m.SpeedKnots)
		fmt.Printf("Course:        %f\n", m.CourseDegrees)
		fmt.Printf("Date:          %s\n", m.Date)
		fmt.Printf("Variation:     %f\n", m.MagneticVariation)
	}
}
```

Output:

```text
$ go run .

Raw sentence:  $GPRMC,220516,A,5133.82,N,00042.24,W,173.8,231.8,130694,004.2,W*70
Checksum OK:   true
Talker:        GP
Time:          22:05:16
Status:        valid
Latitude GPS:  5133.8200
Latitude DMS:  51° 33' 49.20"
Longitude GPS: 0042.2400
Longitude DMS: 0° 42' 14.40"
Speed:         173.800000
Course:        231.800000
Date:          1994-06-13
Variation:     -4.200000
```

Two lines do the whole job. `nmea.ParseSentence` splits the line, verifies the
checksum, and gives you the fields; `nmea.DefaultRegistry.Decode` runs the
decoder for that formatter and gives you back a typed value. `nmea.DataTypeOf`
reports which formatter it was, so one branch serves any sentence type, and the
assertion to `sentences.RMC` is what gives you named fields.

`ParseSentence` does not care where the line came from. A serial port, a
capture file, a UDP datagram, or a string in a test are all the same to it.

`FormatGPS` and `FormatDMS` print the same coordinate in the two forms
receivers and charts use. There are also `FormatCoordinate`, `FormatPosition`,
`FormatLatitude`, `FormatLongitude`, `FormatBearing`, `FormatSpeedKnots`,
`FormatMetres`, and `FormatNauticalMiles` for the common conversions.

If you already have a `Parser` and want one line decoded with the same
checksum and error handling as the rest of the stream, use
`parser.ParseLine(line)`, which returns an `Event` with the value in
`ev.Value`.

### NMEA 4.10 tag blocks

A tag block is a metadata prefix with its own checksum, and it is parsed
whether or not the sentence behind it decodes:

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

func main() {
	sentence := "\\s:Satelite_1,c:1553390539*62\\!AIVDM,1,1,,A,13M@ah0025QdPDTCOl`K6`nV00Sv,0*52"
	s, err := nmea.ParseSentence(sentence)
	if err != nil {
		log.Fatal(err)
	}
	v, err := nmea.DefaultRegistry.Decode(s)
	if err != nil {
		log.Fatal(err)
	}
	m := v.(sentences.VDMVDO)

	fmt.Printf("TAG Block present: %v\n", s.TagBlock.Present)
	fmt.Printf("TAG Block source:  %s\n", s.TagBlock.Source)
	fmt.Printf("TAG Block time:    %s\n", time.Unix(s.TagBlock.Time, 0).UTC())
	fmt.Printf("Address:           %s\n", m.Address())
	fmt.Printf("Payload bytes:     %d\n", len(m.Payload))
}
```

Output:

```text
$ go run .

TAG Block present: true
TAG Block source:  Satelite_1
TAG Block time:    2019-03-24 01:22:19 +0000 UTC
Address:           AIVDM
Payload bytes:     21
```

`TagBlock` also carries `Destination`, `Grouping`, `LineCount`, `RelativeTime`,
`Text`, and `Extra` for any key this library does not name, plus
`ChecksumState`, because the tag block's checksum is separate from the
sentence's. `Present` is what tells a zero value apart from an absent block.

Note the `\\` and `!` framing: a tag block opens with `\`, and `ParseSentence`
accepts `!`-framed AIS sentences as well as `$`-framed ones.

### Custom sentence parser

If you need to parse a message that contains an unsupported sentence type you
can implement and register your own message parser and get yourself unblocked
immediately. Embedding `sentences.Base` is what makes the value satisfy
`nmea.DecodedSentence`, so it routes on `DataType` exactly like a built-in one:

```go
package main

import (
	"fmt"
	"log"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

// XYZ is a sentence this library does not know about.
type XYZ struct {
	sentences.Base
	Value float64
}

func (XYZ) Formatter() string { return "XYZ" }

func (XYZ) Decode(s nmea.Sentence) (any, error) {
	v, err := s.Float(0)
	return XYZ{Base: sentences.Base{Sentence: s}, Value: v}, err
}

func main() {
	p := nmea.New()
	p.Use(XYZ{})

	ev, err := p.ParseLine(nmea.Frame("XYZ,42.5"))
	if err != nil {
		log.Fatal(err)
	}
	m := ev.Value.(XYZ)
	fmt.Printf("formatter: %s\n", m.DataType())
	fmt.Printf("value:     %f\n", m.Value)
	fmt.Printf("raw:       %s\n", m.Raw())
}
```

Output:

```text
$ go run .

formatter: XYZ
value:     42.500000
raw:       $XYZ,42.5*6A
```

`s.Float`, `s.Int`, `s.Time`, `s.Date`, `s.Coordinate`, and `s.Field` do the
parsing for you against the sentence's own fields, and any error they return
already names the field that failed. `nmea.Frame` adds the `$`, computes the
checksum, and appends `*hh`, so the example above produces a sentence a real
receiver could have sent.

If you think your custom message parser could be beneficial to other users we
encourage you to contribute back to the library by submitting a PR and get it
included in the list of supported sentences.

### Read a receiver

This is a complete program. It reads a serial port and logs every position
change, which is normally the first thing you want to see working.

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/device"
)

func main() {
	dev, err := device.Open(device.Config{
		Port:     os.Getenv("GPS_PORT"), // COM3 on Windows, /dev/ttyUSB0 on Linux
		BaudRate: 9600,
		// Without a read timeout a receiver that goes quiet is
		// indistinguishable from one that has hung.
		ReadTimeout: 2 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	// Handlers run on the read loop, so keep them short: copy out what you
	// need and return. An error returned from a handler is reported to the
	// error handlers and does not stop the stream.
	dev.OnFix(func(f *nmea.Fix) error {
		if !f.HasPosition {
			return nil // still searching for satellites
		}
		log.Printf("%.6f, %.6f  alt %.1fm  %v  sats %d/%d  %.1fkn",
			f.Latitude, f.Longitude, f.Altitude, f.Quality,
			f.SatellitesUsed, f.SatellitesInView, f.SpeedKnots)
		return nil
	})

	// Bad checksums and decode failures go here rather than being fatal, so
	// one corrupt sentence does not end the run.
	dev.OnError(func(err error) { log.Printf("nmea: %v", err) })

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Consume blocks until the context is cancelled or the device closes.
	// It returns nil on a clean shutdown.
	if err := dev.Consume(ctx); err != nil {
		log.Fatal(err)
	}
}
```

```sh
GPS_PORT=COM3 go run .
```

Find the ports the system can see:

```sh
nmea-dump -list
```

### File, socket, or any reader

Any `io.Reader` works: a `net.Conn`, a file, a byte buffer. No serial package
is involved, so this path has no external dependencies at all.

```go
parser := nmea.New()

parser.OnFix(func(f *nmea.Fix) error {
	if f.HasPosition {
		log.Printf("%.6f, %.6f", f.Latitude, f.Longitude)
	}
	return nil
})

if err := parser.Consume(ctx, reader); err != nil && ctx.Err() == nil {
	log.Fatal(err)
}
```

Over TCP the receiver sends plain NMEA text, so it is just a reader. Keep the
connection open with reads unblocked, and give the socket its own timeout,
because the parser's read timeout is a serial concept:

```go
conn, err := net.Dial("tcp", "192.168.1.20:10110")
if err != nil {
	log.Fatal(err)
}
defer conn.Close()

go func() {
	<-ctx.Done()
	conn.Close() // release a read blocked on an idle connection
}()

if err := parser.Consume(ctx, conn); err != nil && ctx.Err() == nil {
	log.Fatal(err)
}
```

One sentence that arrives whole, with no stream at all:

```go
ev, err := parser.ParseLine("$GPRMC,123519,A,4807.038,N,01131.000,E,...*6A")
if err != nil {
	log.Fatal(err)
}
if !ev.OK() {
	log.Fatal(ev.Err)
}
```

Or take every decoded event off a channel and process it yourself:

```go
for ev := range parser.Events() {
	// ev.Value is the decoded sentence, ev.Err is any decode failure
}
```

### One sentence type

`Watch` is the readable path: a typed value with no type assertion.

```go
import "github.com/tamalmaity-dev/nmea-go-parser/sentences"

sentences.Watch(parser, "GGA", func(g sentences.GGA) error {
	log.Printf("%.6f, %.6f  quality %v  hdop %.1f  %d sats",
		g.Latitude, g.Longitude, g.Quality, g.HDOP, g.SatellitesUsed)
	return nil
})
```

Route on the value yourself when one handler serves several types:

```go
parser.OnEvent(func(ev nmea.Event) error {
	s, ok := ev.Value.(nmea.DecodedSentence)
	if !ok {
		return nil // not a sentence this build knows
	}
	switch s.DataType() {
	case nmea.TypeRMC:
		m := s.(sentences.RMC)
		log.Printf("RMC %.6f, %.6f", m.Latitude, m.Longitude)
	case nmea.TypeGSV:
		g := s.(sentences.GSV)
		log.Printf("%d satellites in view", g.SatellitesInView)
	}
	return nil
})
```

Every decoded value carries where it came from:

```go
log.Printf("%s from %s: %s", s.DataType(), s.Talker(), s.Constellation())
log.Printf("raw: %s", s.Raw())
```

### The aggregate fix

`Fix` is the merged current state, the answer to "just tell me where it is".
Each sentence fills in the fields it owns and leaves the rest alone.

```go
f := parser.Fix()

f.HasPosition        // a position is available
f.Latitude           // signed decimal degrees, positive north
f.Longitude
f.Valid              // the fix is usable, not merely present
f.Quality            // gps, dgps, rtk-fixed, none, ...
f.NavStatus          // safe, caution, unsafe, not valid (NMEA 4.10)
f.Altitude           // metres above mean sea level
f.SpeedKnots         // with SpeedKmh and SpeedMps precomputed
f.CourseDegrees      // course over ground
f.HeadingDegrees     // and HeadingTrue, because magnetic is not true
f.SatellitesUsed     // from GSA
f.SatellitesInView   // from GSV
f.Satellites         // each with elevation, azimuth, SNR, signal id
f.HorizontalError    // measured, from GST, in metres
```

Every optional value has a `Has*` flag, because a blank NMEA field means
"not available" and that is **not** the same as zero. Latitude `0.0` and
longitude `0.0` are real positions in the Gulf of Guinea, so no sentinel float
is used anywhere. `Fix` is returned by value, so the copy you hold cannot
change under you.

Poll it from your own goroutine, which allocates nothing per sentence:

```go
go func() {
	for range time.Tick(time.Second) {
		f := parser.Fix()
		if f.HasPosition {
			fmt.Println(f.Latitude, f.Longitude)
		}
	}
}()
```

Satellites, joined per system rather than per number:

```go
for _, cs := range f.SatellitesByConstellation() {
	log.Printf("%-8s %2d tracked, %d used", cs.Constellation, cs.Tracked, cs.Used)
}
for _, s := range f.SatelliteStatus() {
	log.Printf("%s used=%v snr=%.0f", s, s.Used, s.SNR)
}
```

### Command line tool

```sh
# List the ports the system can see.
nmea-dump -list

# Read a receiver.
nmea-dump -port COM3 -baud 9600

# Replay the bundled sample capture, so no hardware is needed.
nmea-dump -file testdata/sample.nmea

# Every sentence rather than just position changes.
nmea-dump -raw -file testdata/sample.nmea

# Show the satellite sky after each complete GSV cycle.
nmea-dump -sky -file testdata/sample.nmea

# What this build understands.
nmea-dump -formatters
nmea-dump -version

# Identify the receiver and the NMEA standard version, then exit.
nmea-dump -receiver -file testdata/sample.nmea

# Scripting: stop at the first fix.
nmea-dump -once -port /dev/ttyUSB0

# From a pipe. -file - means standard input.
cat capture.log | nmea-dump -file -
```

| Flag | Default | What it does |
|---|---|---|
| `-port` | first port found | Serial port to read, e.g. `COM3` or `/dev/ttyUSB0` |
| `-baud` | `9600` | Baud rate |
| `-file` | — | Read a file instead of a port; `-` reads standard input |
| `-raw` | off | Print every sentence event, not only position changes |
| `-sky` | off | Print the satellite sky after each complete GSV cycle |
| `-receiver` | off | Report receiver model and NMEA version, then exit |
| `-formatters` | off | List every sentence formatter this build decodes |
| `-list` | off | List available serial ports, then exit |
| `-version` | off | Print this tool's version, then exit |
| `-once` | off | Stop after the first fix, useful for scripting |

A climbing bad-checksum count almost always means the wrong baud rate.

### Command line output

Position changes only are printed by default, because a receiver at 10 Hz
repeats an unchanged position and an unfiltered log is unreadable. Diagnostics
go to **stderr**, so a captured data stream stays clean.

#### Normal

```sh
nmea-dump -file testdata/sample.nmea
```

```text
gps  37.387458, -121.972360 +/-5.0m 7/0 sats 9.0m 16:12:29.487
gps  37.387458, -121.972360 +/-5.0m 7/7 sats 9.0m 0.1kn 310deg 1998-05-12 16:12:29
gps  38.737335,  -9.140638 +/-52.5m 3/7 sats 9.0m 0.1kn 310deg 2004-03-11 11:22:57
gps  38.737335,  -9.140638 +/-1.0m 3/7 sats 9.0m 0.1kn 310deg 2004-03-11 18:21:41
...
gps/safe  44.069006, -121.314327 +/-27.2m 14/12 sats 1113.0m 0.0kn 000deg hdg94true wind10.5kn rot0.0 2015-12-04 00:10:43
NO FIX/safe  53.450657,  -2.240410 +/-27.2m 5/18 sats 71.0m 10.5kn 100deg hdg123mag water4.5kn wind10.0kn rot0.0 2002-12-09 10:36:07
```

Read left to right: fix quality, NMEA navigational status after `/`, latitude
and longitude, horizontal accuracy, satellites used and in view, altitude,
speed over ground, course over ground, heading with its `true`/`mag`
reference, water speed, wind speed, rate of turn, and UTC timestamp. Fields
are only printed once the sentence that supplies them has arrived.

#### Raw

`-raw` adds every sentence event, with decode status and the original line:

```sh
nmea-dump -raw -file testdata/sample.nmea
```

```text
ok   GPGGA      $GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18
ok   GPRMC      $GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10
ok   GPGSA      $GPGSA,A,3,07,02,26,27,09,04,15,,,,,,1.8,1.0,1.5*33
ok   GPGSV      $GPGSV,2,1,07,07,79,048,42,02,51,062,43,26,36,256,42,27,27,138,42*71
ok   GPVTG      $GPVTG,309.62,T, ,M,0.13,N,0.2,K,A*23
...
ERR  MTK        $PMTK001,1,0*32
ok   AIVDM      !AIVDM,1,1,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0*5C
```

`ok` means the sentence decoded successfully. `ERR` means that event had a
decoding or support error, which for the sample capture is four proprietary
lines (`PUBX`, `PMTK`, `PGRME`) that carry no NMEA formatter.

#### Satellite sky

`-sky` prints per-system totals and then every tracked satellite, once per
complete GSV cycle:

```sh
nmea-dump -sky -file testdata/sample.nmea
```

```text
sky: 7 tracked, 7 used
  system    tracked   used  bands
  GPS             7      7  not reported

  sys   prn  band      elev   azim    snr
* GPS     7  -          79°    48°   42dB
* GPS     2  -          51°    62°   43dB
* GPS    26  -          36°   256°   42dB
* GPS    27  -          27°   138°   42dB
* GPS     9  -          23°   313°   42dB
* GPS     4  -          19°   159°   41dB
* GPS    15  -          12°    41°   42dB
  (* = used in the solution)
```

The `*` marks satellites listed in the `GSA` solution. A receiver that reports
its signal ID gives band names; one that does not shows `-`, because there is
nothing to resolve against.

A later cycle in the same capture, with several constellations:

```text
sky: 16 tracked, 5 used (receiver reports 18 in view)
  system    tracked   used  bands
  GPS            10      4  L1 C/A
  GLONASS         1      0  not reported
  Galileo         2      1  not reported
  BeiDou          3      0  not reported
```

The header count and the receiver's own in-view count differ when a GSV cycle
has not fully arrived. Both are shown rather than one being hidden.

#### Stream summary

When the stream ends, a summary goes to stderr:

```text
88 lines, 3644 bytes, 52 sentences contributing to the fix, 0 bad checksums, 4 unknown formatters, 0 events dropped
```

The four unknown formatters are the proprietary sample lines, not missing NMEA
support. A non-zero bad-checksum count means the wrong baud rate or a noisy
line; a non-zero dropped count means a channel consumer fell behind.

## Packages

Six packages, layered so the dependency runs one way and never back:

| Package | Import path | Contains | Depends on |
|---|---|---|---|
| core | `github.com/tamalmaity-dev/nmea-go-parser` | the parser, the decoder registry, the aggregate `Fix`; re-exports the three below | wire, value, fault |
| wire | `github.com/tamalmaity-dev/nmea-go-parser/wire` | framing: `Sentence`, checksums, the line splitter, NMEA 4.10 tag blocks | value, fault |
| value | `github.com/tamalmaity-dev/nmea-go-parser/value` | the values a field carries: `TOD`, `Date`, `Coordinate`, `Version`, `Constellation`, and the formatters | fault |
| fault | `github.com/tamalmaity-dev/nmea-go-parser/fault` | the sentinel errors, and `MaxSentenceLength` | nothing |
| sentences | `github.com/tamalmaity-dev/nmea-go-parser/sentences` | one decoder per sentence formatter, 56 of them | core, wire, value, fault |
| device | `github.com/tamalmaity-dev/nmea-go-parser/device` | serial port handling | core, sentences, `go.bug.st/serial` |

```
fault  ←  value  ←  wire  ←  nmea  ←  sentences  ←  device
```

Framing is separate from values because it has to be exactly right for every
sentence, including ones no decoder has ever seen. A value knows nothing about
sentences, and a fault knows nothing about either, so a decoder can depend on
what a field *means* without depending on the machinery that delivered it.

**You can ignore `wire`, `value` and `fault` entirely.** The core package
re-exports every one of their types, errors, and functions, and because those
are *aliases* rather than redefinitions, `nmea.Coordinate` and
`value.Coordinate` are the same type. So existing code keeps compiling, and a
decoder written against either spelling interoperates with the other. Import
the lower packages directly when you are writing a decoder or a library and
want the layering to be visible.

**Importing `device` is enough.** Its `init` registers every built-in decoder,
so you never import `sentences` unless you want its concrete types. Import it
explicitly for typed values, or when reading from something that is not a
serial port:

```go
import (
	"github.com/tamalmaity-dev/nmea-go-parser"          // core
	"github.com/tamalmaity-dev/nmea-go-parser/sentences" // GGA, RMC, GSV, ...
)
```

The core package knows nothing about any individual sentence. That is what
lets a program use only the decoders it needs, replace a built-in one, or add
a vendor sentence without touching the parser, and why a program reading a
socket or a file never pulls in the serial dependency.

## Documentation

- [docs/nmea.md](docs/nmea.md) — field-by-field reference for all 56 sentences,
  grouped by category, with the standard's own example sentences and notes on
  where equipment gets them wrong.
- `nmea-dump -formatters` — the exact list this build decodes.
- `nmea-dump -h` — every CLI flag.
- `make help` — every build target.

Adding a sentence is one file plus one line: implement `Formatter()` and
`Decode()` in `sentences/<name>.go`, then add it to the `all` slice in
[sentences/sentences.go](sentences/sentences.go), which is the single list
`Install` and the per-file `init` both read.

## Contributing

Please feel free to submit issues or fork the repository and send pull
requests to update the library and fix bugs, implement support for new
sentence types, refactor code, etc.

New sentence types are the most welcome contribution: one file, one line in
`all`, and a test against the sentence's published example. See
[docs/nmea.md](docs/nmea.md) for the field layout and the standard's own
examples.

## License

MIT. See [LICENSE](LICENSE).
