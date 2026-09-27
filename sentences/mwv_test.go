package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestMWVRelativeVersusTrue(t *testing.T) {
	// R is relative to the bow, T is true north. Reading one as the other
	// points the wind in entirely the wrong direction.
	rel := decodeOne[MWV](t, nmea.Frame("WIMWV,214.8,R,10.5,N,A"))
	if !rel.Relative {
		t.Error("an R reference was not reported as relative")
	}
	// Converting needs the vessel's heading.
	if _, ok := rel.TrueDirection(0, false); ok {
		t.Error("a relative angle converted to true without a heading")
	}
	if d, ok := rel.TrueDirection(100, true); !ok || math.Abs(d-314.8) > 1e-6 {
		t.Errorf("TrueDirection = %v, %v; want 314.8, true", d, ok)
	}

	tr := decodeOne[MWV](t, nmea.Frame("WIMWV,180.0,T,0.1,K,V"))
	if tr.Relative {
		t.Error("a T reference was reported as relative")
	}
	if d, ok := tr.TrueDirection(90, true); !ok || d != 180 {
		t.Errorf("TrueDirection = %v, %v; want 180, true", d, ok)
	}
	// The status field says whether the reading may be used at all.
	if tr.Status != nmea.FlagNo {
		t.Errorf("Status = %q, want V", tr.Status)
	}
}
