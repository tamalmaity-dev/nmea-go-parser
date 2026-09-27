package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestRotationDirectionSign(t *testing.T) {
	// The sign is the whole point of ROT: dropping it makes a port turn look
	// like a starboard one.
	port := decodeOne[ROT](t, nmea.Frame("HEROT,-2.5,A"))
	if !port.HasDegrees || math.Abs(port.DegreesPerMinute-(-2.5)) > 1e-9 {
		t.Errorf("DegreesPerMinute = %v, want -2.5", port.DegreesPerMinute)
	}
	if isPort, ok := port.TurningToPort(); !ok || !isPort {
		t.Errorf("TurningToPort = %v, %v; want true, true", isPort, ok)
	}
	if d, ok := port.DegreesPerSecond(); !ok || math.Abs(d-(-2.5/60)) > 1e-9 {
		t.Errorf("DegreesPerSecond = %v, want %v", d, -2.5/60)
	}

	starboard := decodeOne[ROT](t, "$HEROT,0.0,A*2B")
	if _, ok := starboard.TurningToPort(); ok {
		t.Error("a zero rate reported a direction")
	}
}
