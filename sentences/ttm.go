package sentences

import (
	"fmt"
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TTM is the Tracked Target Message: the relative motion of one target, as
// reported by a radar or ARPA tracker.
//
//	$RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,N,SEN,Y*23
//
// Field layout:
//
//	0  target number, 0 to 99
//	1  distance                     2 bearing        3 T true or R relative
//	4  speed                        5 course         6 T true or R relative
//	7  distance of closest approach 8 time to it, "-" meaning increasing
//	9  speed and distance units, K kilometres or N nautical miles
//	10 target name
//	11 target status
//	12 reference
//	13 UTC of the data, NMEA 3.0 and later
//	14 type, A auto, M manual, or R reported, NMEA 3.0 and later
//
// The three reference fields are the ones that matter. Distance and bearing are
// relative to the vessel, speed and course are relative to the water, and the
// closest-approach figures are relative to neither. Reading a relative bearing
// as a true one puts a target tens of degrees out of place, so each is kept
// with its own reference rather than being flattened into a single number.
//
// TTM is not an observation of the vessel's own position, so it never touches
// the fix. A program tracking targets reads the sentence or keeps its own list
// keyed by target number.
type TTM struct {
	Base
	// Number is the tracker's label for this target, which is what TLL and TLB
	// refer back to.
	Number    int
	HasNumber bool

	// Distance and Bearing, with DistanceBearingTrue saying which north the
	// bearing is measured from.
	Distance    float64
	HasDistance bool
	Bearing     float64
	HasBearing  bool
	// DistanceBearingTrue is true for a T reference and false for R.
	DistanceBearingTrue bool
	HasDistanceBearing  bool

	// Speed and Course, with SpeedCourseTrue saying which frame they are in.
	// A target's course over the ground and its course through the water differ
	// by the set and drift, so the reference decides what the number means.
	Speed           float64
	HasSpeed        bool
	Course          float64
	HasCourse       bool
	SpeedCourseTrue bool
	HasSpeedCourse  bool

	// DistanceToCPA and TimeToCPA describe the closest point of approach. A
	// TimeToCPA of "-" means the vessels are diverging, which is a real answer
	// rather than a missing one.
	DistanceToCPA    float64
	HasDistanceToCPA bool
	TimeToCPA        float64
	HasTimeToCPA     bool
	// Diverging is the "-" flag on the time field.
	Diverging bool

	// Units is K for kilometres or N for nautical miles, and applies to both
	// the distance and the speed.
	Units    SpeedUnit
	HasUnits bool

	// Name identifies the target for a display.
	Name string
	// Status is the tracker's own state for the target, whose letters are
	// equipment specific.
	Status string
	// Reference is the third reference field, which the documentation leaves
	// undescribed beyond naming it.
	Reference string

	// UTC and Type arrived in NMEA 3.0.
	UTC     nmea.TOD
	HasUTC  bool
	Type    string
	HasType bool
}

// ApparentBearing is the bearing a radar display shows: the relative one, since
// that is what is drawn relative to the vessel.
func (t TTM) ApparentBearing() (deg float64, ok bool) {
	if !t.HasBearing || t.HasDistanceBearing && t.DistanceBearingTrue {
		return 0, false
	}
	return t.Bearing, true
}

// String renders the track for a log line.
func (t TTM) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "TTM %d", t.Number)
	if t.HasDistance {
		fmt.Fprintf(&b, " %.1f", t.Distance)
		if t.HasUnits {
			if t.Units == SpeedUnitKmh {
				b.WriteString("km")
			} else {
				b.WriteString("nm")
			}
		}
	}
	if t.HasBearing {
		ref := "R"
		if t.DistanceBearingTrue {
			ref = "T"
		}
		fmt.Fprintf(&b, " %03.0f%s", t.Bearing, ref)
	}
	if t.HasSpeed {
		fmt.Fprintf(&b, " %.1fkn", t.Speed)
	}
	if t.Name != "" {
		b.WriteString(" " + t.Name)
	}
	return b.String()
}

type ttm struct{}

func (ttm) Formatter() string { return "TTM" }

func (ttm) Decode(s nmea.Sentence) (any, error) {
	out := TTM{Base: newBase(s)}
	// The target number is what TLL and TLB refer back to, so a sentence
	// without one cannot be matched to anything.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}

	var err error
	if out.Number, out.HasNumber, err = optionalInt(s, 0); err != nil {
		return out, err
	}
	if out.Distance, out.HasDistance, err = optionalFloat(s, 1); err != nil {
		return out, err
	}
	if out.Bearing, out.HasBearing, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.DistanceBearingTrue, out.HasDistanceBearing, err = referenceField(s, 3, "TTM", "distance and bearing"); err != nil {
		return out, err
	}
	if out.Speed, out.HasSpeed, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.Course, out.HasCourse, err = optionalFloat(s, 5); err != nil {
		return out, err
	}
	if out.SpeedCourseTrue, out.HasSpeedCourse, err = referenceField(s, 6, "TTM", "speed and course"); err != nil {
		return out, err
	}
	if out.DistanceToCPA, out.HasDistanceToCPA, err = optionalFloat(s, 7); err != nil {
		return out, err
	}
	// A "-" on the time field means the vessels are diverging. That is a real
	// answer, so it is recorded rather than treated as a parse failure.
	if text(s.Field(8)) == "-" {
		out.Diverging = true
	} else {
		if out.TimeToCPA, out.HasTimeToCPA, err = optionalFloat(s, 8); err != nil {
			return out, err
		}
	}
	if u := text(s.Field(9)); u != "" {
		// The units letter covers both the distance and the speed, so it is
		// resolved once and applied to both.
		switch u {
		case "K", "k":
			out.Units, out.HasUnits = SpeedUnitKmh, true
		case "N", "n":
			out.Units, out.HasUnits = SpeedUnitKnots, true
		default:
			return out, errMislabeled("TTM", "distance and speed units", u)
		}
	}
	out.Name = text(s.Field(10))
	out.Status = text(s.Field(11))
	out.Reference = text(s.Field(12))
	if out.UTC, err = s.Time(13); err == nil {
		out.HasUTC = true
	}
	out.Type = text(s.Field(14))
	out.HasType = out.Type != ""
	return out, nil
}

// referenceField reads a T or R reference letter, reporting true for true. A
// blank field is a normal state for an optional reference and is not an error.
func referenceField(s nmea.Sentence, i int, formatter, which string) (isTrue bool, present bool, err error) {
	switch text(s.Field(i)) {
	case "T", "t":
		return true, true, nil
	case "R", "r":
		return false, true, nil
	case "":
		return false, false, nil
	default:
		return false, false, errMislabeled(formatter, which, text(s.Field(i)))
	}
}
