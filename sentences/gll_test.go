package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGLL(t *testing.T) {
	g := decodeOne[GLL](t, "$GPGLL,3723.2475,N,12158.3416,W,161229.487,A,A*41")

	lat, lon, ok := g.Latitude, g.Longitude, g.HasPosition
	if !ok {
		t.Fatal("Position reported no position")
	}
	if math.Abs(lat-37.3874583) > 1e-6 || math.Abs(lon+121.97236) > 1e-5 {
		t.Errorf("position = %v, %v, want 37.3874583, -121.97236", lat, lon)
	}
	if g.Status != nmea.StatusValid {
		t.Errorf("Status = %v, want valid", g.Status)
	}
	if g.UTC.Hour != 16 || g.UTC.Minute != 12 || g.UTC.Second != 29 {
		t.Errorf("UTC = %v, want 16:12:29", g.UTC)
	}
	if g.Mode != "A" {
		t.Errorf("Mode = %q, want A", g.Mode)
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGLL verifies the six-field position sentence.
func TestPublishedGLL(t *testing.T) {
	// Published example.
	g := published[GLL](t, "GNGLL,4404.14012,N,12118.85993,W,001037.00,A,A")

	if !near(g.Latitude, 44+4.14012/60, 1e-9) {
		t.Errorf("field 0 latitude = %v, want %v", g.Latitude, 44+4.14012/60)
	}
	if !near(g.Longitude, -(121 + 18.85993/60), 1e-9) {
		t.Errorf("field 2 longitude = %v, want %v", g.Longitude, -(121 + 18.85993/60))
	}
	// Field 4: UTC. Field 5: status. Field 6: the FAA mode.
	if g.UTC.Hour != 0 || g.UTC.Minute != 10 || g.UTC.Second != 37 {
		t.Errorf("field 4 UTC = %v, want 00:10:37", g.UTC)
	}
	if g.Status != nmea.StatusValid {
		t.Errorf("field 5 status = %v, want valid", g.Status)
	}
	if g.Mode != "A" {
		t.Errorf("field 6 mode = %q, want A", g.Mode)
	}
}
