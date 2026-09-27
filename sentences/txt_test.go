package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestTXTAccumulator(t *testing.T) {
	// A TXT message is split across sentences, so it is only meaningful
	// reassembled.
	first := decodeOne[TXT](t, nmea.Frame("GNTXT,02,01,02,first part"))
	second := decodeOne[TXT](t, nmea.Frame("GNTXT,02,02,02,second part"))

	var acc TextAccumulator
	msg, complete := acc.Add(first)
	if complete || msg != "" {
		t.Errorf("after the first fragment: %q, %v; want incomplete", msg, complete)
	}
	msg, complete = acc.Add(second)
	if !complete {
		t.Fatal("the message did not complete")
	}
	if msg != "first partsecond part" {
		t.Errorf("message = %q, want the fragments joined", msg)
	}

	// A new sequence must not splice onto the old one.
	third := decodeOne[TXT](t, nmea.Frame("GNTXT,01,01,01,new message"))
	if msg, complete := acc.Add(third); !complete || msg != "new message" {
		t.Errorf("after a new sequence: %q, %v; want %q, true", msg, complete, "new message")
	}
}
func TestXTECrossTrackSign(t *testing.T) {
	x := decodeOne[XTE](t, "$GPXTE,A,A,0.67,L,N*6F")
	if !x.HasCrossTrack || math.Abs(x.CrossTrackSigned-(-0.67)) > 1e-9 {
		t.Errorf("CrossTrackSigned = %v, want -0.67 for a port error", x.CrossTrackSigned)
	}
	if x.Steer != nmea.SideLeft {
		t.Errorf("Steer = %v, want left", x.Steer)
	}
	nm, ok := x.CrossTrackNauticalMiles()
	if !ok || math.Abs(nm-(-0.67)) > 1e-9 {
		t.Errorf("CrossTrackNauticalMiles = %v, %v; want -0.67, true", nm, ok)
	}

	// A warning status with no measurements must still decode.
	warn := decodeOne[XTE](t, "$GPXTE,V,V,,,N,S*43")
	if warn.Status != nmea.StatusInvalid {
		t.Errorf("Status = %v, want invalid", warn.Status)
	}
	if warn.HasCrossTrack {
		t.Error("HasCrossTrack = true for a blank field")
	}
	if warn.Mode != "S" {
		t.Errorf("Mode = %q, want S", warn.Mode)
	}
}
