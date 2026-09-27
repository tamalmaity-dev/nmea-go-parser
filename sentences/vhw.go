package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VHW is the Water Speed and Heading sentence. It reports speed through the
// water, which on a moving vessel or in a current differs from speed over the
// ground, and gives both a true and a magnetic heading alongside it.
//
//	$IIVHW,100.0,T,105.0,M,5.5,N,10.2,K
//
// Field layout:
//
//	0 heading true      degrees
//	1 reference         T
//	2 heading magnetic  degrees
//	3 reference         M
//	4 speed through water knots
//	5 units             N
//	6 speed through water km/h
//	7 units             K
//
// Speed through the water is measured by a transducer or inferred from the
// engine, not by satellites. A program wanting ground speed should use VTG or
// RMC instead, and the difference between the two is the current, which is
// often the more useful of the pair on a working vessel.
type VHW struct {
	Base
	HeadingTrue     float64
	HasHeadingTrue  bool
	HeadingMagnetic float64
	HasHeadingMag   bool
	SpeedKnots      float64
	SpeedKmh        float64
	HasSpeed        bool
}

type vhw struct{}

func (vhw) Formatter() string { return "VHW" }

func (vhw) Decode(s nmea.Sentence) (any, error) {
	// A speed-only or heading-only VHW is valid; the other half may be blank.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := VHW{Base: newBase(s)}

	var err error
	if out.HeadingTrue, out.HasHeadingTrue, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HasHeadingTrue && !referenceOK(s.Field(1), "T") {
		return out, errMislabeled("VHW", "true heading", s.Field(1))
	}
	if out.HeadingMagnetic, out.HasHeadingMag, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.HasHeadingMag && !referenceOK(s.Field(3), "M") {
		return out, errMislabeled("VHW", "magnetic heading", s.Field(3))
	}

	// Knots is the stored figure and km/h is redundant, so reading what the
	// receiver actually sent is better than deriving one from the other: the
	// two disagree slightly because the receiver rounds each independently,
	// and a program comparing them against its own log should see the sent
	// value. The conversion is only used when the receiver omits the km/h
	// field, which some do.
	if out.SpeedKnots, out.HasSpeed, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.HasSpeed && !referenceOK(s.Field(5), "N") {
		return out, errMislabeled("VHW", "water speed", s.Field(5))
	}
	kmh, hasKmh, err := optionalFloat(s, 6)
	if err != nil {
		return out, err
	}
	switch {
	case hasKmh:
		if !referenceOK(s.Field(7), "K") {
			return out, errMislabeled("VHW", "water speed", s.Field(7))
		}
		out.SpeedKmh = kmh
	case out.HasSpeed:
		out.SpeedKmh = out.SpeedKnots * 1.852
	}
	return out, nil
}

// Position returns the vessel's speed and heading for a caller that wants a
// single summary rather than the sentence's own structure.
func (v VHW) Position() (heading float64, speedKnots float64, ok bool) {
	h := v.HeadingTrue
	if !v.HasHeadingTrue {
		h = v.HeadingMagnetic
	}
	if !v.HasSpeed {
		return 0, 0, false
	}
	return h, v.SpeedKnots, true
}

// ApplyFix folds the VHW into the fix state. Water speed is recorded
// separately from the ground speed that VTG and RMC report, because the
// difference between them is the current.
func (v VHW) ApplyFix(f *nmea.Fix) {
	if v.HasHeadingTrue {
		f.HeadingDegrees, f.HasHeading, f.HeadingTrue = v.HeadingTrue, true, true
	} else if v.HasHeadingMag {
		f.HeadingDegrees, f.HasHeading, f.HeadingTrue = v.HeadingMagnetic, true, false
	}
	if v.HasSpeed {
		f.WaterSpeedKnots, f.HasWaterSpeed = v.SpeedKnots, true
		f.WaterSpeedKmh = v.SpeedKnots * 1.852
	}
}
