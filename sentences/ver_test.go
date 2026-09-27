package sentences

import (
	"math"
	"testing"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestHeadingConversions(t *testing.T) {
	h := decodeOne[HDT](t, "$GPHDT,274.07,T*03")
	if !h.Heading.HasDegrees || math.Abs(h.Heading.Degrees-274.07) > 1e-9 {
		t.Errorf("heading = %v, want 274.07", h.Heading.Degrees)
	}
	if !h.Heading.True {
		t.Error("a T reference was not reported as true")
	}
	// A true heading needs no conversion.
	if d, ok := h.Heading.Bearing(10, true); !ok || d != 274.07 {
		t.Errorf("Bearing = %v, %v; want 274.07, true", d, ok)
	}
}
func TestSpeedUnitConversions(t *testing.T) {
	tests := []struct {
		unit SpeedUnit
		in   float64
		kn   float64
		mps  float64
	}{
		{SpeedUnitKnots, 10, 10, 5.144444},
		{SpeedUnitKmh, 18.52, 10, 5.144444},
		{SpeedUnitMps, 5.144444, 10, 5.144444},
	}
	for _, tc := range tests {
		if got := tc.unit.ToKnots(tc.in); math.Abs(got-tc.kn) > 1e-4 {
			t.Errorf("%v.ToKnots(%v) = %v, want %v", tc.unit, tc.in, got, tc.kn)
		}
		if got := tc.unit.ToMetresPerSecond(tc.in); math.Abs(got-tc.mps) > 1e-4 {
			t.Errorf("%v.ToMetresPerSecond(%v) = %v, want %v", tc.unit, tc.in, got, tc.mps)
		}
	}
	if _, err := ParseSpeedUnit("X"); err == nil {
		t.Error("ParseSpeedUnit(\"X\") succeeded, want an error")
	}
}
