package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestBOD(t *testing.T) {
	b := decodeOne[BOD](t, "$GPBOD,097.0,T,103.2,M,POINTB,POINTA*4A")
	if !b.HasBearingTrue || math.Abs(b.BearingTrue-97.0) > 1e-9 {
		t.Errorf("BearingTrue = %v, want 97.0", b.BearingTrue)
	}
	if !b.HasBearingMag || math.Abs(b.BearingMagnetic-103.2) > 1e-9 {
		t.Errorf("BearingMagnetic = %v, want 103.2", b.BearingMagnetic)
	}
	if b.Destination != "POINTB" || b.Origin != "POINTA" {
		t.Errorf("destination/origin = %q/%q, want POINTB/POINTA", b.Destination, b.Origin)
	}
}
func TestBODWithoutOrigin(t *testing.T) {
	// A receiver in goto mode has only a destination, and the standard's own
	// example omits the origin. It must still decode.
	b := decodeOne[BOD](t, "$GPBOD,099.3,T,105.6,M,POINTB*64")
	if b.Destination != "POINTB" {
		t.Errorf("Destination = %q, want POINTB", b.Destination)
	}
	if b.Origin != "" {
		t.Errorf("Origin = %q, want empty", b.Origin)
	}
}
func TestBlankFieldsAreNotErrors(t *testing.T) {
	// NMEA says a field must be left empty when there is no valid data. A
	// decoder that treats a blank as malformed would fail on every sentence a
	// receiver emits before it has a fix.
	lines := []string{
		"$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18",
		"$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10",
		"$GPVTG,309.62,T, ,M,0.13,N,0.2,K,A*23",
		"$GPGGA,103607.00,,,,,0,00,99.9,,,,,,,*70",
		"$GPRMC,103607.00,V,,,,,,,,,,N*7E",
		"$GPRTE,1,1,w,0*13",
		"$GPBOD,099.3,T,105.6,M,POINTB*64",
	}
	for _, line := range lines {
		s, err := nmea.ParseSentence(line)
		if err != nil {
			t.Fatalf("ParseSentence(%q): %v", line, err)
		}
		if _, err := nmea.DefaultRegistry.Decode(s); err != nil {
			t.Errorf("decoding %q: %v", line, err)
		}
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedBOD verifies the field order, in which the destination
// precedes the origin, the reverse of RMB.
func TestPublishedBOD(t *testing.T) {
	// Published example 2, the one with both endpoints.
	b := published[BOD](t, "GPBOD,097.0,T,103.2,M,POINTB,POINTA")

	// Field 0: bearing true. Field 1: its T reference.
	if !near(b.BearingTrue, 97.0, 1e-9) {
		t.Errorf("field 0 true bearing = %v, want 97.0", b.BearingTrue)
	}
	// Field 2: bearing magnetic. Field 3: its M reference.
	if !near(b.BearingMagnetic, 103.2, 1e-9) {
		t.Errorf("field 2 magnetic bearing = %v, want 103.2", b.BearingMagnetic)
	}
	// Field 4 is the DESTINATION and field 5 the ORIGIN.
	if b.Destination != "POINTB" {
		t.Errorf("field 4 = %q, want POINTB (destination)", b.Destination)
	}
	if b.Origin != "POINTA" {
		t.Errorf("field 5 = %q, want POINTA (origin)", b.Origin)
	}

	// Published example 1 omits the origin, which is a complete sentence.
	single := published[BOD](t, "GPBOD,099.3,T,105.6,M,POINTB")
	if single.Origin != "" {
		t.Errorf("origin = %q for the 5-field example, want empty", single.Origin)
	}
	if single.Destination != "POINTB" {
		t.Errorf("destination = %q, want POINTB", single.Destination)
	}
}
