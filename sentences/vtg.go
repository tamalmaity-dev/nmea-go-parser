package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VTG is Course Over Ground and Ground Speed. It reports the same velocity
// as RMC but with more units and both true and magnetic references, and
// unlike RMC it carries no position.

//------------------------------------------------------------------------------------------
//	$GPVTG,054.7,T,034.4,M,005.5,N,010.2,K,A
//------------------------------------------------------------------------------------------

// Field layout:
//
//	0 course true      degrees
//	1 reference        T for true
//	2 course magnetic  degrees
//	3 reference        M for magnetic
//	4 speed            knots
//	5 unit             N for knots
//	6 speed            km/h
//	7 unit             K for km/h
//	8 FAA mode         receiver specific, NMEA 2.3 and later
//
// The reference fields are present to stop a magnetic course being read as a
// true one. They are validated, because a mislabelled heading is a silent
// error that looks like a plausible number.
type VTG struct {
	Base
	// CourseTrue and CourseMagnetic are the two headings, each with a
	// presence flag. Only one is normally populated; older receivers send
	// neither.
	CourseTrue     float64
	HasCourseTrue  bool
	CourseMagnetic float64
	HasCourseMag   bool
	// SpeedKnots and SpeedKmh are both read from the wire rather than one
	// being derived from the other. A receiver computes its km/h figure from
	// a more precise internal value than the three decimals it reports in
	// knots, so the two differ slightly: the published example sends 2.550
	// knots alongside 4.724 km/h, which is 4.7226 by conversion. Keeping both
	// as sent means a program comparing them is not misled by that.
	SpeedKnots    float64
	HasSpeedKnots bool
	SpeedKmh      float64
	HasSpeedKmh   bool
	Mode          string
}

type vtg struct{}

func (vtg) Formatter() string { return "VTG" }

func (vtg) Decode(s nmea.Sentence) (any, error) {
	// A VTG that omits everything is meaningless, but a receiver that has
	// no velocity fix still sends the sentence with blank fields, so only
	// the field count is enforced.
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := VTG{Base: newBase(s)}
	var err error

	// The reference flag must agree with the field it labels, otherwise the
	// number would be silently reinterpreted as the other kind of heading.
	if !referenceOK(s.Field(1), "T") {
		return out, errMislabeled("VTG", "true course", s.Field(1))
	}
	if out.CourseTrue, out.HasCourseTrue, err = optionalFloat(s, 0); err != nil {
		return out, err
	}

	if !referenceOK(s.Field(3), "M") {
		return out, errMislabeled("VTG", "magnetic course", s.Field(3))
	}
	if out.CourseMagnetic, out.HasCourseMag, err = optionalFloat(s, 2); err != nil {
		return out, err
	}

	// Fields 4 to 7: speed in knots and the same speed in km/h. Both are
	// read as sent rather than one being derived, because a receiver's two
	// figures are computed from more internal precision than it prints, so
	// they differ in the last decimal place.
	if out.SpeedKnots, out.HasSpeedKnots, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.SpeedKmh, out.HasSpeedKmh, err = optionalFloat(s, 6); err != nil {
		return out, err
	}

	out.Mode = text(s.Field(8))
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: the trailing mode field
// arrived in NMEA 2.3.
func (v VTG) NMEASupports() []nmea.Feature {
	if v.Mode != "" {
		return nmea.FeatureOnly(nmea.FeatureFAA)
	}
	return nil
}

// ApplyFix folds the VTG into the fix state. Only the true course is applied,
// since a magnetic heading is not interchangeable with one.
//
// The fix stores knots and derives its own km/h and m/s, so the two figures
// the sentence sent are reconciled at this point rather than passed on
// separately.
func (v VTG) ApplyFix(f *nmea.Fix) {
	if v.HasSpeedKnots {
		f.SetSpeedKnots(v.SpeedKnots)
	} else if v.HasSpeedKmh {
		f.SetSpeedKnots(SpeedUnitKmh.ToKnots(v.SpeedKmh))
	}
	if v.HasCourseTrue {
		f.CourseDegrees, f.HasCourse = v.CourseTrue, true
	}
}
