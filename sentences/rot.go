package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// ROT is the Rate of Turn sentence. It reports how fast the vessel is
// rotating, which is what a radar or a towed-fish program needs to predict
// the track of a vessel ahead.
//
//	$HEROT,0.0,A
//	$HEROT,-2.5,A
//
// Field layout:
//
//	0 rate of turn  degrees per minute, negative turns to port
//	1 status        A data valid, V invalid
//
// The sign is the whole point of the sentence: port and starboard turns are
// opposite, and a receiver that drops the sign makes a turn the wrong way
// round. Some receivers use a lone "-" to mean turning to port, with no
// magnitude, which is accepted as zero rotation to port.
type ROT struct {
	Base
	// DegreesPerMinute is positive turning to starboard and negative turning
	// to port.
	DegreesPerMinute float64
	HasDegrees       bool
	// Status is A when the reading is valid.
	Status nmea.StatusFlag
}

// TurningToPort reports the direction, and false when the rate is unknown or
// effectively zero, where the direction is not meaningful.
func (r *ROT) TurningToPort() (port bool, ok bool) {
	if !r.HasDegrees || r.DegreesPerMinute == 0 {
		return false, false
	}
	return r.DegreesPerMinute < 0, true
}

// DegreesPerSecond converts to degrees per second, which is the unit a
// simulation or a control loop usually wants.
func (r *ROT) DegreesPerSecond() (float64, bool) {
	return r.DegreesPerMinute / 60.0, r.HasDegrees
}

type rot struct{}

func (rot) Formatter() string { return "ROT" }

func (rot) Decode(s nmea.Sentence) (any, error) {
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := ROT{Base: newBase(s)}
	out.Status = nmea.ParseStatusFlag(s.Field(1))

	if s.Blank(0) {
		return out, nil
	}
	// A bare "-" means turning to port with no magnitude measured, which is a
	// real thing for a receiver to report and must not fail the sentence.
	if s.Field(0) == "-" {
		out.DegreesPerMinute, out.HasDegrees = 0, true
		return out, nil
	}
	v, present, err := optionalFloat(s, 0)
	if err != nil {
		return out, err
	}
	if present {
		out.DegreesPerMinute, out.HasDegrees = v, true
	}
	return out, nil
}

// ApplyFix folds the ROT into the fix state.
func (r ROT) ApplyFix(f *nmea.Fix) {
	if r.HasDegrees {
		f.RateOfTurn, f.HasRateOfTurn = r.DegreesPerMinute, true
	}
}
