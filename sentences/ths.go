package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// THS is the True Heading and Status sentence. It carries the same heading as
// HDT but adds an explicit indicator of where the heading came from, which
// matters on a vessel with more than one compass: an estimated heading from a
// rate gyro is not the same claim as an autonomous one from a satellite.
//
//	$GPTHS,338.01,A
//	$GPTHS,71.3,E
//
// Field layout:
//
//	0 heading   degrees true
//	1 mode      A autonomous, E estimated, M manual, S simulator, V invalid
//
// THS is not covered by the freely available documentation for older
// revisions, but it is a current sentence and the layout above is confirmed
// by two independent receiver manuals.
type THS struct {
	Base
	Heading Heading
	// Mode is the heading source: A autonomous, E estimated by dead
	// reckoning, M manual input, S simulated, V data not valid.
	Mode nmea.StatusFlag
}

// HeadingMode names the THS mode indicator, which is a distinct alphabet from
// the FAA mode used by GGA and RMC.
func (t *THS) HeadingMode() string {
	switch t.Mode {
	case 'A':
		return "autonomous"
	case 'E':
		return "estimated"
	case 'M':
		return "manual"
	case 'S':
		return "simulated"
	case 'V':
		return "not valid"
	default:
		return "unknown"
	}
}

// Valid reports whether the heading may be trusted. An estimated heading is
// usable but is a dead-reckoning extrapolation, so it is reported as not
// valid for anything that needs a measured direction.
func (t *THS) Valid() bool {
	return t.Mode == 'A' || t.Mode == 'E'
}

type ths struct{}

func (ths) Formatter() string { return "THS" }

func (ths) Decode(s nmea.Sentence) (any, error) {
	out := THS{Base: newBase(s)}
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	// THS headings are true by definition, so the single field is read
	// directly rather than through the shared reference-checking helper.
	v, present, err := optionalFloat(s, 0)
	if err != nil {
		return out, err
	}
	if present {
		if v < 0 || v > 360 {
			return out, errRange("THS", "heading", v, "0..360")
		}
		out.Heading = Heading{Degrees: v, HasDegrees: true, True: true}
	}
	out.Mode = nmea.ParseStatusFlag(s.Field(1))
	return out, nil
}

// ApplyFix folds the THS into the fix state.
func (t THS) ApplyFix(f *nmea.Fix) {
	if !t.Heading.HasDegrees {
		return
	}
	f.HeadingDegrees, f.HasHeading = t.Heading.Degrees, true
	f.HeadingTrue = t.Heading.True
	// A not-valid mode invalidates the heading, and with it the fix if the
	// heading is the only orientation the receiver offers.
	if t.Mode == 'V' {
		f.Valid = false
	}
}
