package sentences

import (
	"testing"
)

// TestPublishedOSD verifies the own ship data layout against the documented
// field list:
//
//	$--OSD,x.x,A,x.x,a,x.x,a,x.x,x.x,a
//
// gpsd publishes no worked example, so the values come from the field list and
// from a real receiver sentence:
//
//	$IIOSD,100.0,A,100.0,T,10.5,N,10.5,0.5,N*4A
func TestPublishedOSD(t *testing.T) {
	o := published[OSD](t, "IIOSD,100.0,A,100.0,T,10.5,N,10.5,0.5,N")

	// Field 0: heading, degrees true. Field 1: the data is valid.
	if !o.HasHeading || !near(o.Heading, 100.0, 1e-9) {
		t.Errorf("field 0 heading = %v (present %v), want 100.0", o.Heading, o.HasHeading)
	}
	if o.Status != 'A' {
		t.Errorf("field 1 status = %q, want A", o.Status)
	}
	// Field 2: the vessel course over ground. Field 3: what it is referenced
	// to, which is a different alphabet from the T and M of a heading.
	if !o.HasCourse || !near(o.Course, 100.0, 1e-9) {
		t.Errorf("field 2 course = %v (present %v), want 100.0", o.Course, o.HasCourse)
	}
	if o.CourseRef != "T" {
		t.Errorf("field 3 course reference = %q, want T", o.CourseRef)
	}
	// Field 4: the speed over ground. Field 5: its reference.
	if !o.HasSpeed || !near(o.Speed, 10.5, 1e-9) {
		t.Errorf("field 4 speed = %v (present %v), want 10.5", o.Speed, o.HasSpeed)
	}
	if o.SpeedRef != "N" {
		t.Errorf("field 5 speed reference = %q, want N", o.SpeedRef)
	}
	// Field 6: the vessel set, which is the direction the water is setting
	// towards. Field 7: the drift speed. Field 8: the unit of that speed.
	if !o.HasSet || !near(o.Set, 10.5, 1e-9) {
		t.Errorf("field 6 set = %v (present %v), want 10.5", o.Set, o.HasSet)
	}
	if !o.HasDrift || !near(o.Drift, 0.5, 1e-9) {
		t.Errorf("field 7 drift = %v (present %v), want 0.5", o.Drift, o.HasDrift)
	}
	if o.SpeedUnit != SpeedUnitKnots {
		t.Errorf("field 8 speed unit = %v, want knots", o.SpeedUnit)
	}
}

// TestOSDSpeedKnotsNormalises checks the conversion, which is the reason to
// keep the unit: the fix stores knots, and a receiver reporting km/h would
// otherwise land in the fix unconverted. The unit is field 8; field 5 is the
// speed reference, a different alphabet, so putting K there would be a
// different mistake.
func TestOSDSpeedKnotsNormalises(t *testing.T) {
	metric := published[OSD](t, "IIOSD,100.0,A,100.0,T,10.5,B,10.5,0.5,K")

	if metric.SpeedUnit != SpeedUnitKmh {
		t.Fatalf("SpeedUnit = %v, want km/h", metric.SpeedUnit)
	}
	knots, ok := metric.SpeedKnots()
	if !ok {
		t.Fatal("SpeedKnots reported no value for a sentence carrying a speed")
	}
	if !near(knots, SpeedUnitKmh.ToKnots(10.5), 1e-9) {
		t.Errorf("SpeedKnots = %v, want %v", knots, SpeedUnitKmh.ToKnots(10.5))
	}
}

// TestOSDAllBlank covers the vessel with no instruments at all, which reports
// the sentence with every field empty. It must decode rather than fail,
// because a program watching OSD should keep listening for the values to
// appear.
func TestOSDAllBlank(t *testing.T) {
	o := published[OSD](t, "IIOSD,,,,,,,,")

	if o.HasHeading || o.HasCourse || o.HasSpeed || o.HasSet || o.HasDrift {
		t.Error("an all-blank OSD reported a measurement")
	}
	if _, ok := o.SpeedKnots(); ok {
		t.Error("SpeedKnots reported a value for an all-blank OSD")
	}
}

// TestOSDRejectsOutOfRangeHeading checks the range guard on the heading, which
// is 0 to 360 degrees.
func TestOSDRejectsOutOfRangeHeading(t *testing.T) {
	if err := publishedErr(t, "IIOSD,361.0,A,100.0,T,10.5,N,10.5,0.5,N"); err == nil {
		t.Error("decoding an OSD heading of 361 degrees succeeded, want an error")
	}
}
