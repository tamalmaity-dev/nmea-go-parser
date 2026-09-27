package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestDPTDepthBelowKeel(t *testing.T) {
	// A transducer at the waterline: keel depth is the reading directly.
	atWaterline := decodeOne[DPT](t, "$INDPT,2.3,0.0*46")
	if d, ok := atWaterline.DepthBelowKeel(); !ok || math.Abs(d-2.3) > 1e-9 {
		t.Errorf("DepthBelowKeel = %v, %v; want 2.3, true", d, ok)
	}

	// A transducer 1.0 m above the waterline: the hull reaches 1.0 m below
	// it, so the reading of 2.3 m means the keel is 1.3 m under water, which
	// is aground rather than a depth.
	above := decodeOne[DPT](t, nmea.Frame("INDPT,2.3,1.0"))
	if d, ok := above.DepthBelowKeel(); ok || math.Abs(d-(-1.3)) > 1e-9 {
		t.Errorf("DepthBelowKeel = %v, %v; want -1.3, false", d, ok)
	}

	// A transducer 0.5 m below the waterline: the offset deepens it.
	below := decodeOne[DPT](t, nmea.Frame("INDPT,2.3,-0.5"))
	if d, ok := below.DepthBelowKeel(); !ok || math.Abs(d-1.8) > 1e-9 {
		t.Errorf("DepthBelowKeel = %v, %v; want 1.8, true", d, ok)
	}
}
func TestDPTGroundedIsNotADepth(t *testing.T) {
	// A negative result means the vessel is aground, and reporting it as a
	// very shallow passage would be worse than reporting nothing.
	d := decodeOne[DPT](t, nmea.Frame("INDPT,3.0,1.0"))
	if _, ok := d.DepthBelowKeel(); ok {
		t.Error("DepthBelowKeel reported a depth for a grounded vessel")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedDPT verifies the two-field form and the optional range
// scale at field 2.
func TestPublishedDPT(t *testing.T) {
	d := published[DPT](t, "INDPT,2.3,0.0")
	if !near(d.Depth, 2.3, 1e-9) {
		t.Errorf("field 0 depth = %v, want 2.3", d.Depth)
	}
	if !near(d.Offset, 0, 1e-9) {
		t.Errorf("field 1 offset = %v, want 0.0", d.Offset)
	}
	if d.HasRangeScale {
		t.Errorf("field 2 range scale = %v, want absent", d.RangeScale)
	}
}
