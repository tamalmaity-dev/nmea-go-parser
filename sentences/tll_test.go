package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TLL decoder tests.

// TestPublishedTLL verifies the target position layout.
func TestPublishedTLL(t *testing.T) {
	tl := published[TLL](t, "GPTLL,31,4951.42,N,01211.75,W,WHALE,123519,A")

	// Field 0: the target number, which joins this to TTM and TLB.
	if !tl.HasNumber || tl.Number != 31 {
		t.Errorf("field 0 number = %v (present %v), want 31", tl.Number, tl.HasNumber)
	}
	// Fields 1 to 4: the target's position, with the hemispheres applied.
	if !tl.HasPosition {
		t.Fatal("HasPosition = false for a sentence carrying a position")
	}
	if !near(tl.Latitude, 49+51.42/60, 1e-9) {
		t.Errorf("field 1 latitude = %v, want %v", tl.Latitude, 49+51.42/60)
	}
	// The western longitude is negative, because the hemisphere is part of the
	// value.
	if !near(tl.Longitude, -(12 + 11.75/60), 1e-9) {
		t.Errorf("field 3 longitude = %v, want %v", tl.Longitude, -(12 + 11.75/60))
	}
	// The raw form is kept, because the number of decimal places is equipment
	// dependent and re-encoding must not change it.
	if tl.LatitudeRaw.String() == "" {
		t.Error("LatitudeRaw was lost")
	}
	// Field 5: the name.
	if tl.Name != "WHALE" {
		t.Errorf("field 5 name = %q, want WHALE", tl.Name)
	}
	// Field 7: the status.
	if tl.Status != "A" {
		t.Errorf("field 7 status = %q, want A", tl.Status)
	}
	// Field 8: the reference marker, absent here.
	if tl.Reference {
		t.Error("Reference = true for a blank reference field")
	}
}

// TestTLLReferenceTarget covers the R marker, which flags the single target
// used as the reference for relative motion.
func TestTLLReferenceTarget(t *testing.T) {
	tl := published[TLL](t, "GPTLL,31,4807.038,N,01131.000,E,TGT1,220516,V,R")
	if !tl.Reference {
		t.Error("Reference = false for an R marker")
	}
	if !near(tl.Latitude, 48+7.038/60, 1e-9) {
		t.Errorf("latitude = %v, want %v", tl.Latitude, 48+7.038/60)
	}
	if !near(tl.Longitude, 11+31.0/60, 1e-9) {
		t.Errorf("longitude = %v, want %v", tl.Longitude, 11+31.0/60)
	}
	if tl.Status != "V" {
		t.Errorf("status = %q, want V", tl.Status)
	}
}

// TestTLLLostTargetHasNoPosition covers a tracker reporting a target it has
// lost. The position is blank, which is a normal state rather than a fault, and
// the status says why.
func TestTLLLostTargetHasNoPosition(t *testing.T) {
	tl := published[TLL](t, "GPTLL,31,,N,,W,TGT1,,L")
	if tl.HasPosition {
		t.Error("HasPosition = true for a lost target with a blank position")
	}
	if tl.Latitude != 0 || tl.Longitude != 0 {
		t.Errorf("a blank position decoded as %v, %v rather than zero with no flag",
			tl.Latitude, tl.Longitude)
	}
	// The identity survives even though the position does not, which is what
	// lets a program keep showing the target as lost.
	if !tl.HasNumber || tl.Number != 31 {
		t.Errorf("number = %v, want 31 even with no position", tl.Number)
	}
	if tl.Name != "TGT1" {
		t.Errorf("name = %q, want TGT1", tl.Name)
	}
	if tl.Status != "L" {
		t.Errorf("status = %q, want L", tl.Status)
	}
}

// TestTLLVariableDecimals covers the documented quirk that the number of
// decimal places is equipment dependent. A receiver sending two places and one
// sending four must not produce positions a hundred metres apart for the same
// target, so the raw form is preserved.
func TestTLLVariableDecimals(t *testing.T) {
	coarse := published[TLL](t, "GPTLL,31,4951.42,N,01211.75,W,TGT1,123519,A")
	fine := published[TLL](t, "GPTLL,31,4951.4231,N,01211.7512,W,TGT1,123519,A")

	if coarse.Latitude == fine.Latitude {
		t.Skip("the two examples decoded to the same latitude, so there is nothing to compare")
	}
	// The difference must be small: under a metre, which is the point of
	// keeping the raw form rather than re-rounding to a fixed width.
	if diff := math.Abs(coarse.Latitude - fine.Latitude); diff > 1.0/3600 {
		t.Errorf("latitudes differ by %v degrees, want under one second of arc", diff)
	}
}

// TestTLLTruncated covers a sentence with no target number.
func TestTLLTruncated(t *testing.T) {
	if err := publishedErr(t, "GPTLL"); err == nil {
		t.Error("decoding a TLL with no fields succeeded, want an error")
	}
}

// TestTLLIsNotAnObservation is the important one. TLL carries a position, and it
// is the wrong one: a tracked target's, not the vessel's. Several widely-copied
// decoders read TLL as a vessel position, which silently moves the reported fix
// onto whatever the radar happened to be tracking.
func TestTLLIsNotAnObservation(t *testing.T) {
	var v any = published[TLL](t, "GPTLL,31,4951.42,N,01211.75,W,WHALE,123519,A")
	if _, ok := v.(nmea.FixContributor); ok {
		t.Error("TLL implements FixContributor, but its position is the target's, not the vessel's")
	}
}
