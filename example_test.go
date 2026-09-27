package nmea_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/device"
	"github.com/tamalmaity-dev/nmea-go-parser/sentences"
)

// Reading a serial port and printing the position is the common case. The
// device package opens the port and brings in every built-in decoder, so the
// only import needed for a working program is the one below.
func Example() {
	dev, err := device.Open(device.Config{
		Port:     "COM3",
		BaudRate: 9600,
		// Without a read timeout, a receiver that goes quiet looks
		// identical to one that has hung, and there is no way to tell.
		ReadTimeout: 2 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer dev.Close()

	dev.OnFix(func(f *nmea.Fix) error {
		if !f.HasPosition {
			return nil // still searching for satellites
		}
		log.Printf("%.6f, %.6f  %v  sats %d/%d",
			f.Latitude, f.Longitude, f.Quality, f.SatellitesUsed, f.SatellitesInView)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Consume blocks, so it runs in its own goroutine. It returns nil on a
	// clean shutdown.
	go dev.Consume(ctx)

	// ... the rest of the program ...
}

// Reading from something that is not a serial port needs only the root
// package, so a program reading a file or a socket never pulls in the serial
// dependency.
func ExampleParser_Consume_file() {
	f, err := os.Open("testdata/sample.nmea")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// The sentences package registers every built-in decoder on import.
	parser := nmea.New()

	// A handler is called on the read loop, so a slow one is a problem. The
	// usual shape is to copy out what matters and return.
	var reported bool
	parser.OnFix(func(f *nmea.Fix) error {
		if reported || !f.HasPosition {
			return nil
		}
		reported = true
		fmt.Printf("%.5f, %.5f\n", f.Latitude, f.Longitude)
		return nil
	})

	if err := parser.Consume(context.Background(), f); err != nil {
		log.Fatal(err)
	}
	// Output:
	// 37.38746, -121.97236
}

// The aggregate fix is the answer to "where is it", so a program that only
// needs the position can poll it instead of subscribing.
func ExampleParser_Fix() {
	parser := nmea.New(
		nmea.WithChannelBuffer(64),
		nmea.WithKeepHistory(8),
	)

	// Any io.Reader works: a file, a socket, a byte buffer.
	stream := strings.NewReader(
		"$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\r\n" +
			"$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10\r\n")

	if err := parser.Consume(context.Background(), stream); err != nil {
		log.Fatal(err)
	}

	f := parser.Fix()
	// Formatted rather than printed raw, because a float64 in binary
	// arithmetic prints as 0.24076000000000003 and that is noise, not a
	// result worth showing.
	fmt.Printf("position: %.6f, %.6f\n", f.Latitude, f.Longitude)
	fmt.Println("quality: ", f.Quality)
	fmt.Printf("speed:    %.2f knots = %.2f km/h\n", f.SpeedKnots, f.SpeedKmh)
	if ts, ok := f.Timestamp(); ok {
		fmt.Println("at:      ", ts.Format(time.RFC3339))
	}

	// The raw sentences are retained for inspection, which is invaluable when
	// a receiver misbehaves.
	for _, ev := range parser.History() {
		fmt.Printf("%s %s\n", ev.Sentence.Address(), ev.Sentence.ChecksumState)
	}
	// Output:
	// position: 37.387458, -121.972360
	// quality:  gps
	// speed:    0.13 knots = 0.24 km/h
	// at:       1998-05-12T16:12:29Z
	// GPGGA valid
	// GPRMC valid
}

// A typed handler gets the decoded value without a type assertion.
func ExampleWatch() {
	parser := nmea.New()

	// GGA is the sentence to watch for a live position report: it carries
	// altitude and the fix-quality indicator alongside the coordinates.
	sentences.Watch(parser, "GGA", func(g sentences.GGA) error {
		log.Printf("%.6f, %.6f  quality %v  hdop %.1f  %d sats",
			g.Latitude, g.Longitude, g.Quality, g.HDOP, g.SatellitesUsed)
		return nil
	})

	// GNS is better than GGA where a receiver sends it: the mode indicator
	// names each constellation contributing to the solution.
	sentences.Watch(parser, "GNS", func(g sentences.GNS) error {
		for _, m := range g.ModePerConstellation() {
			log.Printf("%s: %s", m.Constellation, m.Mode)
		}
		return nil
	})

	stream := strings.NewReader(
		"$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\r\n" +
			"$GPGNS,112257.00,3844.24011,N,00908.43828,W,AN,03,10.5,,*57\r\n")

	if err := parser.Consume(context.Background(), stream); err != nil {
		log.Fatal(err)
	}
}

// Checksums are calculated, verified, and produced. All three are exposed,
// because a program that configures a receiver has to generate them and a
// program that reads one has to check them.
func Example_checksums() {
	body := "GPWPL,4917.16,N,12310.64,W,003"

	// Calculate.
	fmt.Printf("checksum: %02X\n", nmea.Checksum(body))

	// Verify a complete sentence.
	sentence := nmea.Frame(body)
	fmt.Println("sentence: ", sentence)
	fmt.Println("valid:    ", nmea.Valid(sentence))
	if err := nmea.Validate(sentence); err != nil {
		log.Fatal(err)
	}

	// A corrupt sentence is reported, and the sentinels are testable with
	// errors.Is rather than by matching message text.
	corrupt := "$GPWPL,4917.16,N,12310.64,W,004*00"
	_, _, state := nmea.Split(corrupt)
	fmt.Println("corrupt:  ", state, state.Error())

	// An absent checksum is not a failure: receivers omit it often.
	body2, _, state := nmea.Split("$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W")
	fmt.Printf("no checksum: %q %v (usable %v)\n", body2, state, state.OK())
	// Output:
	// checksum: 65
	// sentence:  $GPWPL,4917.16,N,12310.64,W,003*65
	// valid:     true
	// corrupt:   invalid nmea: checksum mismatch
	// no checksum: "GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W" absent (usable true)
}

// The standard version is inferred from the traffic, because almost no
// receiver states it.
func ExampleParser_Version() {
	parser := nmea.New()

	// A NMEA 4.10 receiver: the GSV carries a signal id, the GSA a system
	// id, and the RMC a navigational status.
	stream := strings.NewReader(
		"$GPGSV,3,1,12,03,45,150,42,07,30,210,39,11,62,080,45,14,15,310,35,1*60\r\n" +
			"$GNGSA,A,3,03,07,11,14,,,,,,,,,1.5,0.6,1.2,1*31\r\n")

	if err := parser.Consume(context.Background(), stream); err != nil {
		log.Fatal(err)
	}

	v, stated := parser.Version()
	fmt.Println("detected:", v, "stated by receiver:", stated)

	// Which talkers this build can attribute to a constellation.
	for _, t := range nmea.KnownTalkers() {
		fmt.Printf("%s = %s\n", t, nmea.TalkerConstellation(t))
	}
	// Output:
	// detected: 4.10 stated by receiver: false
	// BD = BeiDou
	// GA = Galileo
	// GB = BeiDou
	// GI = NavIC
	// GL = GLONASS
	// GN = Mixed
	// GP = GPS
	// GQ = QZSS
	// IR = NavIC
	// PQ = QZSS
	// QZ = QZSS
}

// A waypoint read out of a receiver round-trips byte for byte, which is what
// makes it safe to load a route from one device and write it to another.
func ExampleWPL_Encode() {
	sentence, err := nmea.ParseSentence("$GPWPL,4917.16,N,12310.64,W,003*65")
	if err != nil {
		log.Fatal(err)
	}

	decoded, err := nmea.DefaultRegistry.Decode(sentence)
	if err != nil {
		log.Fatal(err)
	}
	w := decoded.(sentences.WPL)

	fmt.Printf("%s is at %.4f, %.4f\n", w.Name, w.Latitude, w.Longitude)

	// Re-encoding produces the identical sentence, checksum included.
	fmt.Println("re-encoded:", w.Encode("GP"))
	fmt.Println("identical: ", w.Encode("GP") == sentence.Raw)
	// Output:
	// 003 is at 49.2860, -123.1773
	// re-encoded: $GPWPL,4917.16,N,12310.64,W,003*65
	// identical:  true
}

// Reading a sentence directly, without a stream, is useful for tests and for
// sentences that arrive by another route such as a UDP packet.
func ExampleParser_ParseLine() {
	parser := nmea.New()

	ev, err := parser.ParseLine("$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A")
	if err != nil {
		log.Fatal(err)
	}
	if !ev.OK() {
		log.Fatalf("sentence failed: %v", ev.Err)
	}

	// The decoded value is available through As, so one handler can serve
	// several sentence types.
	var rmc sentences.RMC
	if err := ev.As(&rmc); err != nil {
		log.Fatal(err)
	}
	fmt.Println(rmc.SpeedKnots, "knots, course", rmc.CourseDegrees, "degrees")

	// And the fix was updated as a side effect.
	f := parser.Fix()
	_, lon, ok := f.DecimalPosition()
	fmt.Printf("longitude %.6f (position present %v)\n", lon, ok)
	// Output:
	// 22.4 knots, course 84.4 degrees
	// longitude 11.516667 (position present true)
}

// The channel is an alternative to handlers, for a program that wants to
// process sentences in its own goroutine.
func ExampleParser_Events() {
	parser := nmea.New(nmea.WithChannelBuffer(16))

	go func() {
		// The events channel is never closed, so a consumer bounds its own
		// loop. Here the stream simply ends.
		_ = parser.Consume(context.Background(), bytes.NewReader(corpusBytes))
	}()

	types := map[string]int{}
	for ev := range parser.Events() {
		types[ev.Sentence.Type]++
	}
	_ = types
}

var corpusBytes = []byte(
	"$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18\r\n" +
		"$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10\r\n")
