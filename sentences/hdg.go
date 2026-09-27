package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// HDG is the Heading, Deviation and Variation sentence. It reports a
// compass heading together with both corrections needed to turn it into a
// true heading: the deviation caused by local magnetic interference aboard
// the vessel, and the variation between magnetic and true north.
//
//	$HCHDG,101.1,,,7.1,W
//	$HCHDG,,,,15.3,E
//	$HCHDG,245.1,2.3,E,7.1,W
//
// Field layout:
//
//	0 magnetic heading  degrees
//	1 deviation         degrees, magnetic to the compass, optional
//	2 deviation dir     E or W, optional
//	3 variation         degrees, magnetic to true, optional
//	4 variation dir     E or W, optional
//
// Deviation and variation are frequently blank, and a heading with no
// correction is still a usable heading: it is simply a magnetic one. Both
// are signed by their direction field, east positive.
//
// The distinction from THS matters: this is a magnetic compass reading, not
// a satellite-derived heading, and on a vessel with local interference the
// two can disagree by several degrees.
type HDG struct {
	Base
	// Magnetic is the compass heading in degrees.
	Magnetic    float64
	HasMagnetic bool
	// Deviation is the local magnetic error in degrees, signed east
	// positive: the amount to add to the compass to get the true magnetic
	// reading.
	Deviation    float64
	HasDeviation bool
	// Variation is the angle between magnetic and true north in degrees,
	// signed east positive.
	Variation    float64
	HasVariation bool
}

// TrueHeading returns the heading in degrees true, applying deviation and
// then variation, and false when the correction cannot be completed.
//
// Deviation is subtracted because a compass needle is deflected by the
// vessel's own steel; variation is then added to reach true north. Returning
// false for a magnetic heading with no variation is deliberate: a charting
// program that silently treated a magnetic heading as true would be wrong by
// the local variation, which can exceed 20 degrees.
func (h *HDG) TrueHeading() (deg float64, ok bool) {
	if !h.HasMagnetic {
		return 0, false
	}
	if !h.HasVariation {
		return 0, false
	}
	deg = h.Magnetic
	if h.HasDeviation {
		deg -= h.Deviation
	}
	deg += h.Variation
	for deg >= 360 {
		deg -= 360
	}
	for deg < 0 {
		deg += 360
	}
	return deg, true
}

// MagneticHeading returns the compass reading in degrees, which needs no
// correction to be reported as a magnetic heading.
func (h *HDG) MagneticHeading() (deg float64, ok bool) { return h.Magnetic, h.HasMagnetic }

type hdg struct{}

func (hdg) Formatter() string { return "HDG" }

func (hdg) Decode(s nmea.Sentence) (any, error) {
	// A receiver with no compass still emits HDG with everything blank, so an
	// entirely empty sentence is valid. Only a completely absent field list
	// is a failure.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := HDG{Base: newBase(s)}

	if out.Magnetic, out.HasMagnetic, _ = optionalFloat(s, 0); out.HasMagnetic {
		if out.Magnetic < 0 || out.Magnetic > 360 {
			return out, errRange("HDG", "magnetic heading", out.Magnetic, "0..360")
		}
	}
	if out.Deviation, out.HasDeviation, _ = optionalFloat(s, 1); out.HasDeviation {
		if s.Field(2) == "W" {
			out.Deviation = -out.Deviation
		}
	}
	if out.Variation, out.HasVariation, _ = optionalFloat(s, 3); out.HasVariation {
		if s.Field(4) == "W" {
			out.Variation = -out.Variation
		}
	}
	return out, nil
}

// ApplyFix folds the HDG into the fix state.
//
// A magnetic heading is not stored as a true one. When both the correction
// and the heading are present the true heading is derived, and otherwise only
// the magnetic value is recorded with HeadingTrue false, so a caller can see
// which it has.
func (h HDG) ApplyFix(f *nmea.Fix) {
	if h.HasVariation {
		f.MagneticVariation, f.HasMagVariation = h.Variation, true
	}
	if deg, ok := h.TrueHeading(); ok {
		f.HeadingDegrees, f.HasHeading, f.HeadingTrue = deg, true, true
		return
	}
	if h.HasMagnetic {
		f.HeadingDegrees, f.HasHeading, f.HeadingTrue = h.Magnetic, true, false
	}
}
