package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestVTG(t *testing.T) {
	v := decodeOne[VTG](t, "$GPVTG,309.62,T, ,M,0.13,N,0.2,K,A*23")

	if !v.HasCourseTrue || math.Abs(v.CourseTrue-309.62) > 1e-9 {
		t.Errorf("CourseTrue = %v (present %v), want 309.62", v.CourseTrue, v.HasCourseTrue)
	}
	// The magnetic course is a space, not a number, so it must be absent
	// rather than zero.
	if v.HasCourseMag {
		t.Errorf("HasCourseMag = true for a blank field, want false")
	}
	if !v.HasSpeedKnots || math.Abs(v.SpeedKnots-0.13) > 1e-9 {
		t.Errorf("SpeedKnots = %v (present %v), want 0.13", v.SpeedKnots, v.HasSpeedKnots)
	}
	if v.Mode != "A" {
		t.Errorf("Mode = %q, want A", v.Mode)
	}
}
func TestVTGMislabelledReferenceIsRejected(t *testing.T) {
	// A magnetic course sitting in the true slot would otherwise be read as
	// a true heading: a plausible number, silently wrong by 10 to 20 degrees.
	line := nmea.Frame("GPVTG,34.4,M,34.4,M,0.13,N,0.2,K,A")
	s, err := nmea.ParseSentence(line)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nmea.DefaultRegistry.Decode(s); err == nil {
		t.Error("a magnetic course in the true slot decoded without error, want an error")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedVTG verifies the interleaved value and reference fields,
// and that a blank magnetic track is absent rather than zero.
func TestPublishedVTG(t *testing.T) {
	// Published example.
	v := published[VTG](t, "GPVTG,220.86,T,,M,2.550,N,4.724,K,A")

	// Field 0: course over ground true. Field 1: its T reference.
	if !near(v.CourseTrue, 220.86, 1e-9) {
		t.Errorf("field 0 true track = %v, want 220.86", v.CourseTrue)
	}
	// Fields 2 and 3: the magnetic course, blank here. A zero would be a
	// due-north magnetic heading, which is a different claim.
	if v.HasCourseMag {
		t.Errorf("field 2 magnetic track = %v, want absent", v.CourseMagnetic)
	}
	// Fields 4 and 5: speed in knots. Fields 6 and 7: the same in km/h.
	if !near(v.SpeedKnots, 2.550, 1e-9) {
		t.Errorf("field 4 speed = %v, want 2.550", v.SpeedKnots)
	}
	if !near(v.SpeedKmh, 4.724, 1e-9) {
		t.Errorf("field 6 speed = %v, want 4.724", v.SpeedKmh)
	}
	// Field 8: the FAA mode, A.
	if v.Mode != "A" {
		t.Errorf("field 8 mode = %q, want A", v.Mode)
	}
}
