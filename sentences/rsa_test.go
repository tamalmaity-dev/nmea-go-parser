package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestRSA(t *testing.T) {
	// A two-rudder vessel: the mean is the steering angle and the
	// half-difference is the yaw the hull experiences.
	r := decodeOne[RSA](t, "$IIRSA,10.5,A,-3.2,A*58")
	mean, yaw, ok := r.RudderAngle()
	if !ok || math.Abs(mean-3.65) > 1e-9 || math.Abs(yaw-6.85) > 1e-9 {
		t.Errorf("RudderAngle = %v, %v, %v; want 3.65, 6.85, true", mean, yaw, ok)
	}

	// A single-rudder vessel leaves the port fields blank, and requiring both
	// would miss every such boat.
	single := decodeOne[RSA](t, nmea.Frame("IIRSA,10.5,A,,V"))
	mean, yaw, ok = single.RudderAngle()
	if !ok || math.Abs(mean-10.5) > 1e-9 || yaw != 0 {
		t.Errorf("RudderAngle = %v, %v, %v; want 10.5, 0, true", mean, yaw, ok)
	}
}
