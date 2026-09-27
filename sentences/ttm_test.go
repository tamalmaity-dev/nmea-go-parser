package sentences

import (
	"strings"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TTM decoder tests.

// TestPublishedTTM verifies the tracked target layout, including the three
// reference fields, which are the ones a decoder most easily flattens.
func TestPublishedTTM(t *testing.T) {
	tg := published[TTM](t, "RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,N,SEN,Y")

	// Field 0: the target number, which is what TLL and TLB refer back to.
	if !tg.HasNumber || tg.Number != 11 {
		t.Errorf("field 0 number = %v (present %v), want 11", tg.Number, tg.HasNumber)
	}
	// Fields 1 to 3: distance and bearing, with the bearing's reference.
	if !tg.HasDistance || tg.Distance != 11.4 {
		t.Errorf("field 1 distance = %v (present %v), want 11.4", tg.Distance, tg.HasDistance)
	}
	if !tg.HasBearing || tg.Bearing != 13.6 {
		t.Errorf("field 2 bearing = %v (present %v), want 13.6", tg.Bearing, tg.HasBearing)
	}
	if !tg.HasDistanceBearing || !tg.DistanceBearingTrue {
		t.Errorf("field 3 reference: true = %v (present %v), want true, true",
			tg.DistanceBearingTrue, tg.HasDistanceBearing)
	}
	// Fields 4 to 6: speed and course, with their own separate reference.
	if !tg.HasSpeed || tg.Speed != 11.2 {
		t.Errorf("field 4 speed = %v (present %v), want 11.2", tg.Speed, tg.HasSpeed)
	}
	if !tg.HasCourse || tg.Course != 13.8 {
		t.Errorf("field 5 course = %v (present %v), want 13.8", tg.Course, tg.HasCourse)
	}
	if !tg.HasSpeedCourse || !tg.SpeedCourseTrue {
		t.Errorf("field 6 reference: true = %v (present %v), want true, true",
			tg.SpeedCourseTrue, tg.HasSpeedCourse)
	}
	// Fields 7 and 8: the closest point of approach.
	if !tg.HasDistanceToCPA || tg.DistanceToCPA != 3.3 {
		t.Errorf("field 7 distance to CPA = %v (present %v), want 3.3",
			tg.DistanceToCPA, tg.HasDistanceToCPA)
	}
	if !tg.HasTimeToCPA || tg.TimeToCPA != 7.0 {
		t.Errorf("field 8 time to CPA = %v (present %v), want 7.0", tg.TimeToCPA, tg.HasTimeToCPA)
	}
	if tg.Diverging {
		t.Error("Diverging = true for a numeric time to CPA")
	}
	// Field 9: the units letter, which covers both distance and speed.
	if !tg.HasUnits || tg.Units != SpeedUnitKnots {
		t.Errorf("field 9 units = %v (present %v), want knots", tg.Units, tg.HasUnits)
	}
	// Field 10: the target name.
	if tg.Name != "SEN" {
		t.Errorf("field 10 name = %q, want SEN", tg.Name)
	}
}

// TestTTMDivergingIsAnAnswer covers the "-" on the time to closest approach.
// It means the vessels are separating, which is a real and important answer
// rather than a missing one, so treating it as blank would lose it.
func TestTTMDivergingIsAnAnswer(t *testing.T) {
	tg := published[TTM](t, "RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,-,N,SEN,Y")

	if !tg.Diverging {
		t.Error("Diverging = false for a - time to CPA")
	}
	if tg.HasTimeToCPA {
		t.Error("HasTimeToCPA = true for a - time to CPA")
	}
	// Everything else must still have arrived.
	if !tg.HasBearing || tg.Bearing != 13.6 {
		t.Errorf("bearing = %v, want 13.6", tg.Bearing)
	}
}

// TestTTMRelativeBearing covers the R reference, which is what a radar
// display draws and the most common case.
func TestTTMRelativeBearing(t *testing.T) {
	tg := published[TTM](t, "RATTM,11,11.4,13.6,R,11.2,13.8,R,3.3,7.0,N,SEN,Y")

	if tg.DistanceBearingTrue {
		t.Error("DistanceBearingTrue = true for an R reference")
	}
	if tg.SpeedCourseTrue {
		t.Error("SpeedCourseTrue = true for an R reference")
	}
	// The relative bearing is what a display shows.
	if deg, ok := tg.ApparentBearing(); !ok || deg != 13.6 {
		t.Errorf("ApparentBearing = %v, %v; want 13.6, true", deg, ok)
	}
	// A true bearing is not an apparent one, so the helper declines it rather
	// than returning a number a radar display would draw wrongly.
	true_ := published[TTM](t, "RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,N,SEN,Y")
	if _, ok := true_.ApparentBearing(); ok {
		t.Error("ApparentBearing returned a value for a true-north bearing")
	}
}

// TestTTMUnitsKilometres covers the other units letter, which switches both the
// distance and the speed scale.
func TestTTMUnitsKilometres(t *testing.T) {
	tg := published[TTM](t, "RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,K,SEN,Y")
	if !tg.HasUnits || tg.Units != SpeedUnitKmh {
		t.Errorf("units = %v (present %v), want km/h", tg.Units, tg.HasUnits)
	}
	if got := tg.String(); !strings.Contains(got, "km") {
		t.Errorf("String = %q, want it to show kilometres", got)
	}
	// A unit letter this library does not know is rejected, because it decides
	// the scale of every number in the sentence.
	if err := publishedErr(t, "RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,X,SEN,Y"); err == nil {
		t.Error("decoding a TTM with unit X succeeded, want an error")
	}
}

// TestTTMRejectsBadReference covers the three reference letters. A relative
// bearing read as a true one puts a target tens of degrees out of place.
func TestTTMRejectsBadReference(t *testing.T) {
	if err := publishedErr(t, "RATTM,11,11.4,13.6,X,11.2,13.8,T,3.3,7.0,N,SEN,Y"); err == nil {
		t.Error("decoding a TTM with bearing reference X succeeded, want an error")
	}
	if err := publishedErr(t, "RATTM,11,11.4,13.6,T,11.2,13.8,X,3.3,7.0,N,SEN,Y"); err == nil {
		t.Error("decoding a TTM with speed reference X succeeded, want an error")
	}
}

// TestTTMTruncated covers a sentence with no target number, which is what ties
// it to TLL and TLB.
func TestTTMTruncated(t *testing.T) {
	if err := publishedErr(t, "RATTM"); err == nil {
		t.Error("decoding a TTM with no fields succeeded, want an error")
	}
}

// TestTTMIsNotAnObservation checks that a tracked target cannot move the fix.
// The target's position and motion say nothing about where the vessel is.
func TestTTMIsNotAnObservation(t *testing.T) {
	var v any = published[TTM](t, "RATTM,11,11.4,13.6,T,11.2,13.8,T,3.3,7.0,N,SEN,Y")
	if _, ok := v.(nmea.FixContributor); ok {
		t.Error("TTM implements FixContributor, but a target is not the vessel")
	}
}
