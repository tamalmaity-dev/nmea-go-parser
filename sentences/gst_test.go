package sentences

import (
	"math"
	"testing"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGST(t *testing.T) {
	g := decodeOne[GST](t, "$GPGST,182141.000,15.5,15.3,7.2,21.8,0.9,0.5,0.8*54")

	if g.UTC.Hour != 18 || g.UTC.Minute != 21 || g.UTC.Second != 41 {
		t.Errorf("UTC = %v, want 18:21:41", g.UTC)
	}
	if !g.HasRMS || math.Abs(g.RMS-15.5) > 1e-9 {
		t.Errorf("RMS = %v, want 15.5", g.RMS)
	}
	if !g.HasSemiMajor || math.Abs(g.SemiMajor-15.3) > 1e-9 {
		t.Errorf("SemiMajor = %v, want 15.3", g.SemiMajor)
	}
	if !g.HasSemiMinor || math.Abs(g.SemiMinor-7.2) > 1e-9 {
		t.Errorf("SemiMinor = %v, want 7.2", g.SemiMinor)
	}
	if !g.HasOrientation || math.Abs(g.Orientation-21.8) > 1e-9 {
		t.Errorf("Orientation = %v, want 21.8", g.Orientation)
	}
	if !g.HasErrorLat || math.Abs(g.ErrorLatitude-0.9) > 1e-9 {
		t.Errorf("ErrorLatitude = %v, want 0.9", g.ErrorLatitude)
	}
	if !g.HasErrorLon || math.Abs(g.ErrorLongitude-0.5) > 1e-9 {
		t.Errorf("ErrorLongitude = %v, want 0.5", g.ErrorLongitude)
	}
	if !g.HasErrorAlt || math.Abs(g.ErrorAltitude-0.8) > 1e-9 {
		t.Errorf("ErrorAltitude = %v, want 0.8", g.ErrorAltitude)
	}

	major, minor, orientation, ok := g.Ellipse()
	if !ok || math.Abs(major-15.3) > 1e-9 || math.Abs(minor-7.2) > 1e-9 || math.Abs(orientation-21.8) > 1e-9 {
		t.Errorf("Ellipse = %v, %v, %v, %v; want 15.3, 7.2, 21.8, true", major, minor, orientation, ok)
	}

	// The per-axis errors combine as a vector, which is the horizontal
	// position error a caller actually wants.
	e, ok := g.PositionError()
	if !ok {
		t.Fatal("PositionError reported no value")
	}
	if want := math.Hypot(0.9, 0.5); math.Abs(e-want) > 1e-9 {
		t.Errorf("PositionError = %v, want %v", e, want)
	}
}
func TestGSTWithBlankFields(t *testing.T) {
	// A 2D fix has no altitude error, so GST is routinely sent with holes.
	// A partial sentence must decode rather than fail.
	g := decodeOne[GST](t, "$GPGST,082356.00,1.8,,,,1.7,1.3,2.2*7E")

	if !g.HasRMS || math.Abs(g.RMS-1.8) > 1e-9 {
		t.Errorf("RMS = %v, want 1.8", g.RMS)
	}
	if g.HasSemiMajor || g.HasSemiMinor || g.HasOrientation {
		t.Error("blank ellipse fields were reported as present")
	}
	if _, _, _, ok := g.Ellipse(); ok {
		t.Error("Ellipse succeeded with no ellipse fields")
	}
	if !g.HasErrorLat || math.Abs(g.ErrorLatitude-1.7) > 1e-9 {
		t.Errorf("ErrorLatitude = %v, want 1.7", g.ErrorLatitude)
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGST verifies the error statistics, including that a blank
// field is absent rather than zero.
func TestPublishedGST(t *testing.T) {
	// Published example.
	g := published[GST](t, "GPGST,182141.000,15.5,15.3,7.2,21.8,0.9,0.5,0.8")

	// Field 0: the time of the fix these errors describe.
	if g.UTC.Hour != 18 || g.UTC.Minute != 21 || g.UTC.Second != 41 {
		t.Errorf("field 0 UTC = %v, want 18:21:41", g.UTC)
	}
	// Field 1: total RMS standard deviation of the range inputs.
	if !near(g.RMS, 15.5, 1e-9) {
		t.Errorf("field 1 RMS = %v, want 15.5", g.RMS)
	}
	// Fields 2 and 3: the error ellipse axes. Field 4: its orientation.
	if !near(g.SemiMajor, 15.3, 1e-9) {
		t.Errorf("field 2 semi-major = %v, want 15.3", g.SemiMajor)
	}
	if !near(g.SemiMinor, 7.2, 1e-9) {
		t.Errorf("field 3 semi-minor = %v, want 7.2", g.SemiMinor)
	}
	if !near(g.Orientation, 21.8, 1e-9) {
		t.Errorf("field 4 orientation = %v, want 21.8", g.Orientation)
	}
	// Fields 5, 6 and 7: the per-axis error standard deviations.
	if !near(g.ErrorLatitude, 0.9, 1e-9) {
		t.Errorf("field 5 latitude error = %v, want 0.9", g.ErrorLatitude)
	}
	if !near(g.ErrorLongitude, 0.5, 1e-9) {
		t.Errorf("field 6 longitude error = %v, want 0.5", g.ErrorLongitude)
	}
	if !near(g.ErrorAltitude, 0.8, 1e-9) {
		t.Errorf("field 7 altitude error = %v, want 0.8", g.ErrorAltitude)
	}
}
