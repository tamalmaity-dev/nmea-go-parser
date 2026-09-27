package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestHDGTrueHeading(t *testing.T) {
	// Deviation is the vessel's own magnetic error and is subtracted;
	// variation is then added to reach true north.
	h := decodeOne[HDG](t, "$HCHDG,101.1,,,7.1,W*3C")
	if !h.HasMagnetic || math.Abs(h.Magnetic-101.1) > 1e-9 {
		t.Errorf("Magnetic = %v, want 101.1", h.Magnetic)
	}
	if h.HasDeviation {
		t.Error("HasDeviation = true for a blank field")
	}
	// West variation is negative.
	if !h.HasVariation || math.Abs(h.Variation-(-7.1)) > 1e-9 {
		t.Errorf("Variation = %v, want -7.1", h.Variation)
	}
	d, ok := h.TrueHeading()
	if !ok || math.Abs(d-94.0) > 1e-9 {
		t.Errorf("TrueHeading = %v, %v; want 94.0, true", d, ok)
	}
}
func TestHDGNoVariationIsNotTrue(t *testing.T) {
	// A compass heading with no variation is a magnetic heading, and saying
	// otherwise would be wrong by the local variation.
	h := decodeOne[HDG](t, nmea.Frame("HCHDG,245.1,,,,"))
	if _, ok := h.TrueHeading(); ok {
		t.Error("TrueHeading succeeded without a variation, want false")
	}
	if d, ok := h.MagneticHeading(); !ok || math.Abs(d-245.1) > 1e-9 {
		t.Errorf("MagneticHeading = %v, %v; want 245.1, true", d, ok)
	}
}
