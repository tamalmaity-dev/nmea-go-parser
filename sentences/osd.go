package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// OSD is the Own Ship Data sentence: the vessel's own heading, course, speed,
// set, and drift, as opposed to the position that GNSS reports.
//
//	$IIOSD,100.0,A,100.0,T,10.5,N,10.5,0.5,N
//
// Field layout:
//
//	0 heading        degrees true
//	1 status         A valid, V invalid
//	2 course         degrees true
//	3 course ref     B, M, W, R, or P
//	4 speed          in the unit of field 5
//	5 speed ref      B, M, W, R, or P
//	6 set            degrees true, the direction the water is setting towards
//	7 drift          speed of the current, in the unit of field 8
//	8 speed units    K km/h, N knots
//
// The reference letters in fields 3 and 5 say what the figure is measured
// relative to or through: B bottom, M magnetic, W water, R radar, P
// position. The unit field only permits kilometres per hour or knots, so a
// value read as m/s would be wrong by nearly a factor of two.
type OSD struct {
	Base
	Heading    float64
	HasHeading bool
	Status     nmea.StatusFlag
	Course     float64
	HasCourse  bool
	CourseRef  string
	Speed      float64
	HasSpeed   bool
	SpeedRef   string
	Set        float64
	HasSet     bool
	Drift      float64
	HasDrift   bool
	SpeedUnit  SpeedUnit
}

// SpeedKnots returns the speed normalised to knots, which is the unit the fix
// stores, and false when the sentence carried no speed.
func (o *OSD) SpeedKnots() (knots float64, ok bool) {
	if !o.HasSpeed {
		return 0, false
	}
	return o.SpeedUnit.ToKnots(o.Speed), true
}

// DriftKnots returns the rate of the current in knots.
func (o *OSD) DriftKnots() (knots float64, ok bool) {
	if !o.HasDrift {
		return 0, false
	}
	return o.SpeedUnit.ToKnots(o.Drift), true
}

type osd struct{}

func (osd) Formatter() string { return "OSD" }

func (osd) Decode(s nmea.Sentence) (any, error) {
	// A vessel with no heading sensor or no log sends OSD with everything
	// blank, so only an absent field list fails.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := OSD{Base: newBase(s)}

	var err error
	if out.Heading, out.HasHeading, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HasHeading {
		if out.Heading < 0 || out.Heading > 360 {
			return out, errRange("OSD", "heading", out.Heading, "0..360")
		}
	}
	out.Status = nmea.ParseStatusFlag(s.Field(1))

	if out.Course, out.HasCourse, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	out.CourseRef = text(s.Field(3))
	if out.Speed, out.HasSpeed, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	out.SpeedRef = text(s.Field(5))

	if out.Set, out.HasSet, err = optionalFloat(s, 6); err != nil {
		return out, err
	}
	if out.Drift, out.HasDrift, err = optionalFloat(s, 7); err != nil {
		return out, err
	}
	if out.SpeedUnit, err = ParseSpeedUnit(s.Field(8)); err != nil {
		return out, err
	}
	return out, nil
}

// ApplyFix folds the OSD into the fix state. The vessel's own course, set,
// and drift describe its motion through the water, which the position fix
// does not otherwise carry.
func (o OSD) ApplyFix(f *nmea.Fix) {
	if o.HasHeading {
		f.HeadingDegrees, f.HasHeading, f.HeadingTrue = o.Heading, true, true
	}
	if o.HasCourse {
		f.CourseDegrees, f.HasCourse = o.Course, true
	}
	if k, ok := o.SpeedKnots(); ok {
		f.SetSpeedKnots(k)
	}
	if o.HasSet {
		f.Set, f.HasSet = o.Set, true
	}
	if k, ok := o.DriftKnots(); ok {
		f.DriftKnots, f.HasDrift = k, true
	}
}
