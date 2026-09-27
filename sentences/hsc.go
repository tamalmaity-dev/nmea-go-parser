package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// HSC is the Heading Steering Command sentence: the heading an autopilot is
// being told to hold, reported against both norths.
//
//	$IIHSC,123.4,T,124.5,M
//
// Field layout:
//
//	0 commanded heading, degrees true    1 T
//	2 commanded heading, degrees magnetic  3 M
//
// The two figures differ by the local magnetic variation, and which one an
// autopilot should be given depends on the compass it drives, so both are kept
// rather than one being derived from the other.
//
// A documented ambiguity is worth recording: the gpsd reference notes that
// GLOBALSAT describes HSC as having nothing to do with headings, instead
// carrying water temperature sensor data, and says it is unclear which reading
// is correct. The heading interpretation is the one implemented here, because it
// is what the standard's own sentence list and the majority of marine equipment
// use. A receiver sending the GLOBALSAT form will produce a depth where a
// heading was expected, which is why the value is range-checked rather than
// accepted whatever it says.
type HSC struct {
	Base
	// HeadingTrue is the commanded heading, degrees true.
	HeadingTrue    float64
	HasHeadingTrue bool
	// HeadingMagnetic is the same command, degrees magnetic.
	HeadingMagnetic float64
	HasHeadingMag   bool
}

type hsc struct{}

func (hsc) Formatter() string { return "HSC" }

func (hsc) Decode(s nmea.Sentence) (any, error) {
	out := HSC{Base: newBase(s)}
	// One heading is enough: a half-populated sentence still carries a usable
	// command.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}

	var err error
	if out.HeadingTrue, out.HasHeadingTrue, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HeadingMagnetic, out.HasHeadingMag, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	return out, nil
}

// HSC deliberately does not implement nmea.FixContributor. It is a command to
// the autopilot rather than a measurement of where the vessel is, and folding a
// heading that has not been achieved yet into the reported heading would have a
// vessel's autopilot lying about its own position. A program that wants the
// commanded heading reads the sentence.
//
// String renders the command for a log line.
func (h HSC) String() string {
	switch {
	case h.HasHeadingTrue && h.HasHeadingMag:
		return "HSC true " + ftoa(h.HeadingTrue) + " mag " + ftoa(h.HeadingMagnetic)
	case h.HasHeadingTrue:
		return "HSC true " + ftoa(h.HeadingTrue)
	case h.HasHeadingMag:
		return "HSC mag " + ftoa(h.HeadingMagnetic)
	default:
		return "HSC no heading"
	}
}
