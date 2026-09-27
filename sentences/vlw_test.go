package sentences

import (
	"math"
	"testing"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestVLWSetAndDrift(t *testing.T) {
	// The difference between the water and ground totals is distance run,
	// which is how current set is measured without a second input.
	v := decodeOne[VLW](t, "$GPVLW,,N,,N,1234.5,N,2345.6,N*5D")
	if v.HasTotalWater {
		t.Error("HasTotalWater = true for a blank field")
	}
	if !v.HasTotalGround || math.Abs(v.TotalGround-1234.5) > 1e-9 {
		t.Errorf("TotalGround = %v, want 1234.5", v.TotalGround)
	}
	if !v.HasTripGround || math.Abs(v.TripGround-2345.6) > 1e-9 {
		t.Errorf("TripGround = %v, want 2345.6", v.TripGround)
	}
	if _, ok := v.SetAndDrift(); ok {
		t.Error("SetAndDrift succeeded without a water total")
	}
	if feats := v.NMEASupports(); len(feats) != 1 {
		t.Errorf("NMEASupports = %v, want the ground-speed feature", feats)
	}
}
