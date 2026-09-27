package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// MWV is the Wind Speed and Angle sentence. It reports the wind relative to
// the vessel's own heading, which is what a sailor on a moving boat actually
// feels, unlike MWD which is referenced to north.
//
//	$WIMWV,214.8,R,10.5,N,A
//	$WIMWV,180.0,T,0.1,K,V
//
// Field layout:
//
//	0 wind angle   0-359 degrees, relative to the bow unless the next field
//	               says otherwise
//	1 reference    R relative to the bow, T relative to true north
//	2 wind speed   in the unit of field 3
//	3 units        N knots, K km/h, M m/s
//	4 status       A valid, V invalid
//
// The reference field is the one that trips people up: a relative angle and a
// true angle are both "0 to 359", and reading one as the other points the
// wind in entirely the wrong direction.
type MWV struct {
	Base
	// AngleDegrees is the wind angle. Its meaning depends on Relative.
	AngleDegrees float64
	HasAngle     bool
	// Relative is true when the angle is measured from the bow, false when
	// it is from true north.
	Relative bool
	// SpeedKnots is the reading normalised to knots.
	SpeedKnots float64
	HasSpeed   bool
	SpeedUnit  SpeedUnit
	// Status is A when the reading is valid.
	Status nmea.StatusFlag
}

// MetresPerSecond returns the wind speed in m/s.
func (m *MWV) MetresPerSecond() (float64, bool) {
	if !m.HasSpeed {
		return 0, false
	}
	return m.SpeedUnit.ToMetresPerSecond(m.SpeedKnots), true
}

// TrueDirection converts a bow-relative angle to degrees true, given the
// vessel's heading. It reports false for an angle that is already true, and
// for a relative angle when the heading is unknown, rather than returning a
// number measured from the wrong datum.
//
// MWV carries a full 0 to 360 circle relative to the bow with no side letter, so
// it needs only the addition and the wrap. VWR and VWT carry 0 to 180 plus a
// side and sign it first; that difference is why the two are separate methods
// rather than one shared helper.
func (m *MWV) TrueDirection(headingDegrees float64, hasHeading bool) (deg float64, ok bool) {
	if !m.HasAngle {
		return 0, false
	}
	if !m.Relative {
		return m.AngleDegrees, true
	}
	if !hasHeading {
		return 0, false
	}
	return normaliseBearing(m.AngleDegrees + headingDegrees), true
}

type mwv struct{}

func (mwv) Formatter() string { return "MWV" }

func (mwv) Decode(s nmea.Sentence) (any, error) {
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := MWV{Base: newBase(s)}

	var err error
	if out.AngleDegrees, out.HasAngle, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HasAngle {
		if out.AngleDegrees < 0 || out.AngleDegrees > 360 {
			return out, errRange("MWV", "wind angle", out.AngleDegrees, "0..360")
		}
	}

	// R for relative to the bow, T for true north. The two are opposites, so
	// an unrecognised letter must not default to either.
	switch s.Field(1) {
	case "R", "r":
		out.Relative = true
	case "T", "t":
		out.Relative = false
	case "":
		// Absent reference: leave Relative false and let the caller decide.
	default:
		return out, errMislabeled("MWV", "wind angle", s.Field(1))
	}

	if out.SpeedKnots, out.HasSpeed, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.SpeedUnit, err = ParseSpeedUnit(s.Field(3)); err != nil {
		return out, err
	}
	if out.HasSpeed {
		out.SpeedKnots = out.SpeedUnit.ToKnots(out.SpeedKnots)
	}
	out.Status = nmea.ParseStatusFlag(s.Field(4))
	return out, nil
}

// ApplyFix folds the MWV into the fix state. The wind direction is only
// stored when the receiver gave a true-north angle, since a bow-relative
// angle is not a compass direction and storing it as one would be wrong.
func (m MWV) ApplyFix(f *nmea.Fix) {
	if m.HasSpeed {
		f.WindSpeedKnots, f.HasWindSpeed = m.SpeedKnots, true
	}
	if deg, ok := m.TrueDirection(f.HeadingDegrees, f.HasHeading); ok && !m.Relative {
		f.WindDirection, f.HasWindDirection = deg, true
	}
}
