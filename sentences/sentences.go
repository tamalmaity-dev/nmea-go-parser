// Package sentences implements the NMEA 0183 sentence decoders.
//
// The package registers every sentence type with nmea.DefaultRegistry when
// it is imported, so a single blank import makes all of them available to
// any parser:
//
//	import _ "github.com/tamalmaity-dev/nmea-go-parser/sentences"
//
// The decoders are grouped by what they are for rather than by protocol
// chapter:
//
//   - Position: GGA, RMC, GLL, VTG. These carry the fix.
//   - Constellations: GSA, GSV, ZDA. These describe fix quality and time.
//   - Navigation: WPL, RTE, RMB, APB, AAM, BOD. These describe routes,
//     waypoints, and bearing or cross-track guidance toward them.
//
// Each decoder lives in its own file and is listed once in the all slice,
// so adding a sentence means adding one file and one line, with no changes
// to the parser.
//
// # Field index convention
//
// Field indices in this package follow NMEA 0183 field numbering, where
// index 0 is the first field after the sentence formatter. So GGA field 5
// is the fix-quality field and field 8 is the altitude. The comments on
// each decoder list the indices with their meaning, which is the only
// documentation of the layout that actually matters when reading these
// files.
//
// # Extending
//
// A vendor-specific or proprietary sentence needs no changes here. Write a
// decoder, register it, and use it:
//
//	type Vendor struct {
//	    Base
//	    Value float64
//	}
//
//	func (Vendor) Formatter() string { return "XYZ" }
//	func (Vendor) Decode(s nmea.Sentence) (any, error) {
//	    v, err := s.Float(0)
//	    return Vendor{Base: Base{Sentence: s}, Value: v}, err
//	}
//
//	p := nmea.New()
//	p.Use(Vendor{})
//
// Embedding Base rather than the bare nmea.Sentence is what makes the value
// satisfy nmea.DecodedSentence, so a Vendor routes on DataType and prints as
// its wire form exactly like a built-in sentence does.
package sentences

import (
	"fmt"
	"math"
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Install registers every built-in decoder on a registry. Importing this
// package already does that for nmea.DefaultRegistry; call Install when
// building a private registry by hand and wanting the full set in it.
func Install(r *nmea.Registry) {
	if r == nil {
		return
	}
	for _, d := range all {
		r.MustRegister(d)
	}
}

// all is the list of built-in decoders, grouped by what they are for. It
// is the single source of truth that Install, init, and Formatters read.
var all = []nmea.Decoder{
	// Position and fix quality.
	gga{}, gns{}, rmc{}, gll{}, vtg{},
	// Constellations, satellites, and integrity.
	gsa{}, gsv{}, gbs{}, grs{}, gst{}, zda{},
	// Heading.
	hdt{}, ths{}, hdg{}, hdm{}, hsc{}, rpm{},
	// Speed, distance, and rate.
	vhw{}, vbw{}, vlw{}, rot{},
	// Navigation and cross-track.
	wpl{}, rte{}, rmb{}, apb{}, aam{}, bod{},
	xte{}, bwc{}, bwr{}, wcv{}, ttm{}, tll{}, tlb{},
	// Environment and depth.
	dpt{}, dbt{}, dbs{}, dbk{}, mtw{}, mda{}, mta{}, mwd{}, mwv{}, vwr{}, vwt{}, dtm{}, xdr{},
	// Vessel.
	osd{}, rsa{},
	// Text and receiver identification.
	txt{}, ver{},
	// AIS and alerting.
	ack{}, abm{}, bbm{}, vdm{}, vdo{},
}

// init registers every built-in decoder with the default registry, which is
// what makes a blank import of this package sufficient to enable them all.
func init() { Install(nmea.DefaultRegistry) }

// Formatters returns every sentence formatter this package can decode,
// which is handy for a CLI that wants to report what it understands.
func Formatters() []string {
	return nmea.DefaultRegistry.Types()
}

// Watch registers a typed handler for one sentence formatter. It is the
// readable way to consume a specific sentence without type-asserting
// inside a generic callback:
//
//	sentences.Watch(parser, "GGA", func(g sentences.GGA) error {
//	    log.Println(g.Latitude, g.Longitude)
//	    return nil
//	})
//
// The decoded value is passed by value, so a handler cannot mutate parser
// state by writing through the pointer. A decoder that still returns *T is
// dereferenced first, so both shapes reach the callback as a T.
//
// The handler is called on the read loop, so keep it short.
func Watch[T any](p *nmea.Parser, formatter string, fn func(T) error) {
	if p == nil || fn == nil {
		return
	}
	var zero T
	p.OnValue(formatter, zero, func(v any) error {
		switch typed := v.(type) {
		case T:
			return fn(typed)
		case *T:
			if typed == nil {
				return fmt.Errorf("sentences: %s decoded to a nil *%T", formatter, zero)
			}
			return fn(*typed)
		default:
			return fmt.Errorf("sentences: %s decoded to %T, want %T", formatter, v, zero)
		}
	})
}

// needFields returns a descriptive error when a sentence is too short for
// the layout its decoder expects.
func needFields(s nmea.Sentence, n int) error {
	return fmt.Errorf("sentences: %s needs at least %d fields, got %d: %w",
		s.Address(), n, len(s.Fields), nmea.ErrFieldCount)
}

// optionalFloat reads a field a receiver is allowed to leave blank. A blank
// field yields 0, false, and no error, which is the NMEA convention for "not
// available" rather than a malformed sentence. Returning the presence flag
// matters because 0.0 is a legitimate value for altitude and for magnetic
// variation.
func optionalFloat(s nmea.Sentence, i int) (v float64, present bool, err error) {
	if s.Blank(i) {
		return 0, false, nil
	}
	v, err = s.Float(i)
	return v, err == nil, err
}

// optionalInt is optionalFloat for integer fields.
func optionalInt(s nmea.Sentence, i int) (v int, present bool, err error) {
	if s.Blank(i) {
		return 0, false, nil
	}
	v, err = s.Int(i)
	return v, err == nil, err
}

// errRequired reports a field the sentence cannot be decoded without that
// arrived blank. It is distinct from needFields, which reports a sentence that
// is too short: "GPHDT,," has the field and leaves it empty, so calling that a
// field count problem sends a caller looking for a truncation that is not
// there.
func errRequired(formatter, which string) error {
	return fmt.Errorf("sentences: %s %s field is empty, and the sentence cannot be decoded without it: %w",
		formatter, which, nmea.ErrEmptyField)
}

// errMislabeled reports a reference flag that contradicts the field it
// labels, such as a magnetic course in the true slot of a VTG.
//
// This is checked rather than ignored because the failure is silent
// otherwise: the number looks entirely plausible, it is just the wrong
// reference, and a compass error of 10 to 20 degrees is easy to miss until
// something has gone badly wrong.
func errMislabeled(formatter, which, got string) error {
	return fmt.Errorf("sentences: %s %s value is labelled %q: %w",
		formatter, which, got, nmea.ErrFieldValue)
}

// errRange reports a field that parsed cleanly but fell outside the range
// the specification allows. Which field is named matters: a caller chasing a
// data problem needs to know which of a sentence's numbers to distrust.
func errRange(formatter, field string, got float64, allowed string) error {
	return fmt.Errorf("sentences: %s %s is %g, outside %s: %w",
		formatter, field, got, allowed, nmea.ErrFieldRange)
}

// referenceOK reports whether a T/M style reference flag is absent or
// matches the expected value.
func referenceOK(got, want string) bool { return got == "" || got == want }

// text trims trailing NULs and spaces from a free-text field such as a
// waypoint name, which receivers pad inconsistently.
func text(s string) string {
	return strings.TrimRight(s, "\x00 ")
}

// hypot is math.Hypot, and abs is math.Abs, kept here so the sentences that
// combine a latitude and a longitude error, or scan for a worst-case
// residual, do not each import math for a single call.
func hypot(a, b float64) float64 { return math.Hypot(a, b) }

func abs(v float64) float64 { return math.Abs(v) }

func atan2(y, x float64) float64 { return math.Atan2(y, x) }

const pi = math.Pi

// knotsToKmh and knotsToMps are the two standard conversions, in one place
// so no sentence can disagree with another about them.
func knotsToKmh(knots float64) float64 { return knots * 1.852 }

func knotsToMps(knots float64) float64 { return knots * 1852.0 / 3600.0 }
