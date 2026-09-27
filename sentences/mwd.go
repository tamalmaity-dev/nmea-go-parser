package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// MWD is the Wind Direction and Speed sentence, which gives both figures
// twice: once relative to true north and once relative to magnetic, each with
// a speed in two units.
//
//	$WIMWD,302.4,T,289.6,M,10.5,N,5.4,M
//	$WIMWD,,,,10.5,N,5.4,M
//
// Field layout:
//
//	0 wind direction true      degrees
//	1 reference                T true, M magnetic
//	2 wind direction magnetic  degrees
//	3 reference                T or M
//	4 wind speed               in the unit of field 5
//	5 units                    N knots, K km/h, M m/s
//	6 wind speed               in the unit of field 7
//	7 units                    N, K, or M
//
// Direction is the direction the wind is coming from, not the direction it is
// blowing towards, which is the convention every weather report uses. The
// difference matters: a wind direction of 90 is an easterly, meaning the wind
// blows towards the west.
type MWD struct {
	Base
	DirectionTrue     float64
	HasDirectionTrue  bool
	DirectionMagnetic float64
	HasDirectionMag   bool
	// SpeedKnots is the reading normalised to knots from whichever unit pair
	// the receiver used, so a program does not have to know.
	SpeedKnots    float64
	HasSpeed      bool
	SpeedUnit     SpeedUnit
	SpeedKnotsAlt float64
	HasSpeedAlt   bool
	SpeedUnitAlt  SpeedUnit
}

// ApparentWindDirection returns the direction the wind is coming from in
// degrees, preferring the true figure.
func (m *MWD) ApparentWindDirection() (deg float64, ok bool) {
	switch {
	case m.HasDirectionTrue:
		return m.DirectionTrue, true
	case m.HasDirectionMag:
		return m.DirectionMagnetic, true
	default:
		return 0, false
	}
}

// MetresPerSecond returns the wind speed in m/s.
func (m *MWD) MetresPerSecond() (float64, bool) {
	if !m.HasSpeed {
		return 0, false
	}
	return m.SpeedUnit.ToMetresPerSecond(m.SpeedKnots), true
}

type mwd struct{}

func (mwd) Formatter() string { return "MWD" }

func (mwd) Decode(s nmea.Sentence) (any, error) {
	// A wind vane that is disconnected or a sensor that has failed leaves
	// every field blank, so an empty sentence is valid.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := MWD{Base: newBase(s)}

	var err error
	if out.DirectionTrue, out.HasDirectionTrue, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HasDirectionTrue {
		if s.Field(1) != "" && s.Field(1) != "T" {
			return out, errMislabeled("MWD", "true wind direction", s.Field(1))
		}
	}
	if out.DirectionMagnetic, out.HasDirectionMag, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.HasDirectionMag {
		if s.Field(3) != "" && s.Field(3) != "M" {
			return out, errMislabeled("MWD", "magnetic wind direction", s.Field(3))
		}
	}

	if out.SpeedKnots, out.HasSpeed, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.SpeedUnit, err = ParseSpeedUnit(s.Field(5)); err != nil {
		return out, err
	}
	if out.HasSpeed {
		// Normalise to knots so the stored figure is comparable with every
		// other sentence in the package.
		out.SpeedKnots = out.SpeedUnit.ToKnots(out.SpeedKnots)
	}
	if out.SpeedKnotsAlt, out.HasSpeedAlt, err = optionalFloat(s, 6); err != nil {
		return out, err
	}
	if out.SpeedUnitAlt, err = ParseSpeedUnit(s.Field(7)); err != nil {
		return out, err
	}
	return out, nil
}

// ApplyFix folds the MWD into the fix state.
func (m MWD) ApplyFix(f *nmea.Fix) {
	if m.HasSpeed {
		f.WindSpeedKnots, f.HasWindSpeed = m.SpeedKnots, true
	}
	if deg, ok := m.ApparentWindDirection(); ok {
		f.WindDirection, f.HasWindDirection = deg, true
	}
}
