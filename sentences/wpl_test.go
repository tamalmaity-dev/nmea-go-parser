package sentences

import (
	"math"
	"strings"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestWPL(t *testing.T) {
	w := decodeOne[WPL](t, "$GPWPL,4917.16,N,12310.64,W,003*65")

	if w.Name != "003" {
		t.Errorf("Name = %q, want 003", w.Name)
	}
	lat, lon, ok := w.Latitude, w.Longitude, w.HasPosition
	if !ok {
		t.Fatal("Position reported no position")
	}
	if math.Abs(lat-49.286) > 1e-3 || math.Abs(lon+123.1773) > 1e-3 {
		t.Errorf("position = %v, %v, want 49.286, -123.1773", lat, lon)
	}
}
func TestWPLRoundTrip(t *testing.T) {
	// Reading a waypoint out of a receiver and writing it back must be
	// byte-identical, or a route is silently corrupted on every round trip.
	const original = "$GPWPL,4917.16,N,12310.64,W,003*65"
	w := decodeOne[WPL](t, original)
	if got := w.Encode("GP"); got != original {
		t.Errorf("Encode = %s, want %s", got, original)
	}

	// The generic talker is the default.
	if got := w.Encode(""); !strings.HasPrefix(got, "$GPWPL,") {
		t.Errorf("Encode(\"\") = %s, want a GP-prefixed sentence", got)
	}
	// Whatever it produces must itself validate.
	if err := nmea.Validate(w.Encode("GP")); err != nil {
		t.Errorf("Validate(Encode(...)) = %v, want nil", err)
	}
}
