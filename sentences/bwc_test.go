package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestBWCAndBWRShareALayout(t *testing.T) {
	// The two sentences are byte for byte identical and differ only in the
	// distance model, so the decoder is shared and the model is recorded.
	gc := decodeOne[BWC](t, nmea.Frame("GPBWC,081837,4917.24,N,12309.57,W,051.9,T,031.6,M,1.3,N,DEST"))
	if gc.Model != GreatCircle {
		t.Errorf("BWC Model = %q, want %q", gc.Model, GreatCircle)
	}
	if !gc.HasBearingTrue || math.Abs(gc.BearingTrue-51.9) > 1e-9 {
		t.Errorf("BearingTrue = %v, want 51.9", gc.BearingTrue)
	}
	if !gc.HasBearingMag || math.Abs(gc.BearingMagnetic-31.6) > 1e-9 {
		t.Errorf("BearingMagnetic = %v, want 31.6", gc.BearingMagnetic)
	}
	if !gc.HasDistance || math.Abs(gc.Distance-1.3) > 1e-9 {
		t.Errorf("Distance = %v, want 1.3", gc.Distance)
	}
	if gc.WaypointID != "DEST" {
		t.Errorf("WaypointID = %q, want DEST", gc.WaypointID)
	}

	rl := decodeOne[BWC](t, nmea.Frame("GPBWR,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,4.6,N,POINT"))
	if rl.Model != RhumbLine {
		t.Errorf("BWR Model = %q, want %q", rl.Model, RhumbLine)
	}
	if !rl.HasDistance || math.Abs(rl.Distance-4.6) > 1e-9 {
		t.Errorf("BWR Distance = %v, want 4.6", rl.Distance)
	}
}
func TestNewSentencesDoNotMoveThePosition(t *testing.T) {
	// Only the observation sentences may set a position. Guidance, waypoint,
	// and vessel sentences describe where other things are or how the vessel
	// is moving, and folding any of them in would move the reported fix.
	mustNot := []string{"XTE", "BWC", "BWR", "WCV", "HDT", "THS", "HDG",
		"VHW", "VBW", "VLW", "ROT", "DPT", "MTW", "MWD", "MWV", "DTM",
		"OSD", "RSA", "TXT", "VER", "GBS", "GRS", "GST"}

	for _, typ := range mustNot {
		p := nmea.New()
		feedSentence(t, p, firstLineOfType(t, typ))
		if p.Fix().HasPosition {
			t.Errorf("%s set a position on the fix, which it must never do", typ)
		}
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedBWC verifies the bearing and distance layout, shared with
// BWR.
func TestPublishedBWC(t *testing.T) {
	// Published example 2, which the source prints without a leading '$'.
	b := published[BWC](t, "GPBWC,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,0004.6,N,EGLM")

	// Field 0: UTC.
	if b.UTC.Hour != 22 || b.UTC.Minute != 5 || b.UTC.Second != 16 {
		t.Errorf("field 0 UTC = %v, want 22:05:16", b.UTC)
	}
	// Fields 1 to 4: the waypoint position, 51 deg 30.02' N and
	// 0 deg 46.34' W. The hemisphere is part of the value, so the western
	// longitude is negative.
	if !near(b.Latitude, 51+30.02/60, 1e-9) {
		t.Errorf("field 1 latitude = %v, want %v", b.Latitude, 51+30.02/60)
	}
	if !near(b.Longitude, -(0 + 46.34/60), 1e-9) {
		t.Errorf("field 3 longitude = %v, want %v", b.Longitude, -(0 + 46.34/60))
	}
	// Fields 5 to 8: the two bearings with their references.
	if !near(b.BearingTrue, 213.8, 1e-9) {
		t.Errorf("field 5 true bearing = %v, want 213.8", b.BearingTrue)
	}
	if !near(b.BearingMagnetic, 218.0, 1e-9) {
		t.Errorf("field 7 magnetic bearing = %v, want 218.0", b.BearingMagnetic)
	}
	// Field 9: distance in nautical miles. Field 11: the waypoint name.
	if !near(b.Distance, 4.6, 1e-9) {
		t.Errorf("field 9 distance = %v, want 4.6", b.Distance)
	}
	if b.WaypointID != "EGLM" {
		t.Errorf("field 11 waypoint = %q, want EGLM", b.WaypointID)
	}
	if b.Model != GreatCircle {
		t.Errorf("Model = %q, want %q", b.Model, GreatCircle)
	}

	// BWR has a byte-identical layout, so the same decoder serves it and
	// only the distance model differs.
	r := published[BWR](t, "GPBWR,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,0004.6,N,EGLM")
	if r.Model != RhumbLine {
		t.Errorf("BWR Model = %q, want %q", r.Model, RhumbLine)
	}
	if !near(r.BearingTrue, 213.8, 1e-9) {
		t.Errorf("BWR true bearing = %v, want 213.8", r.BearingTrue)
	}
}
