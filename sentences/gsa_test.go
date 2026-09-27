package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGSA(t *testing.T) {
	g := decodeOne[GSA](t, "$GPGSA,A,3,07,02,26,27,09,04,15,,,,,,1.8,1.0,1.5*33")

	if g.Mode != nmea.FlagYes {
		t.Errorf("Mode = %q, want A for automatic", g.Mode)
	}
	if !g.HasFixType || g.FixType != 3 {
		t.Errorf("FixType = %d (present %v), want 3", g.FixType, g.HasFixType)
	}
	if d := g.Dimensions(); d != Degrees3 {
		t.Errorf("Dimensions = %v, want 3D", d)
	}
	// Fields 2 to 13 are the twelve satellite slots, here five of which are
	// used and the rest blank.
	want := []int{7, 2, 26, 27, 9, 4, 15}
	if len(g.SatelliteIDs) != len(want) {
		t.Fatalf("SatelliteIDs = %v, want %v", g.SatelliteIDs, want)
	}
	for i := range want {
		if g.SatelliteIDs[i] != want[i] {
			t.Errorf("SatelliteIDs[%d] = %d, want %d", i, g.SatelliteIDs[i], want[i])
		}
	}
	// PDOP, HDOP, and VDOP are fields 14, 15, and 16 in that order. A
	// decoder that reads them as VDOP/HDOP/PDOP produces plausible numbers
	// with the wrong meaning.
	if !g.HasPDOP || math.Abs(g.PDOP-1.8) > 1e-9 {
		t.Errorf("PDOP = %v, want 1.8", g.PDOP)
	}
	if !g.HasHDOP || math.Abs(g.HDOP-1.0) > 1e-9 {
		t.Errorf("HDOP = %v, want 1.0", g.HDOP)
	}
	if !g.HasVDOP || math.Abs(g.VDOP-1.5) > 1e-9 {
		t.Errorf("VDOP = %v, want 1.5", g.VDOP)
	}
}
func TestGSASystemID(t *testing.T) {
	// Field 17 is the NMEA 4.10 system id, optional in practice.
	withID := decodeOne[GSA](t, "$GNGSA,A,3,03,07,11,14,,,,,,,,,1.5,0.6,1.2,1*31")
	if !withID.HasTrailingField {
		t.Error("HasTrailingField = false for a 4.10 GSA")
	}
	if !withID.HasSystemID || withID.SystemID != nmea.SystemIDGPS {
		t.Errorf("SystemID = %v (present %v), want GPS", withID.SystemID, withID.HasSystemID)
	}
	// The DOP fields are 14, 15, 16 and must not be shifted by the extra
	// field.
	if !withID.HasPDOP || math.Abs(withID.PDOP-1.5) > 1e-9 {
		t.Errorf("PDOP = %v, want 1.5", withID.PDOP)
	}
	if !withID.HasHDOP || math.Abs(withID.HDOP-0.6) > 1e-9 {
		t.Errorf("HDOP = %v, want 0.6", withID.HDOP)
	}
	if !withID.HasVDOP || math.Abs(withID.VDOP-1.2) > 1e-9 {
		t.Errorf("VDOP = %v, want 1.2", withID.VDOP)
	}

	// The same receiver also emits GSA without the field.
	withoutID := decodeOne[GSA](t, "$GPGSA,A,3,04,06,07,09,11,20,30,,,,,,1.8,0.9,1.5*3B")
	if withoutID.HasTrailingField {
		t.Error("HasTrailingField = true for a pre-4.10 GSA")
	}
	if !withoutID.HasPDOP || math.Abs(withoutID.PDOP-1.8) > 1e-9 {
		t.Errorf("PDOP = %v, want 1.8", withoutID.PDOP)
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGSA verifies the twelve satellite slots and the three
// dilution figures, whose order is the thing decoders most often transpose.
func TestPublishedGSA(t *testing.T) {
	// Published example.
	g := published[GSA](t, "GNGSA,A,3,80,71,73,79,69,,,,,,,,1.83,1.09,1.47")

	// Field 0: automatic. Field 1: 3D fix.
	if g.Mode != nmea.FlagYes {
		t.Errorf("field 0 mode = %q, want A", g.Mode)
	}
	if g.Dimensions() != Degrees3 {
		t.Errorf("field 1 fix type = %v, want 3D", g.Dimensions())
	}
	// Fields 2 to 13: five used satellite slots and seven blank ones.
	want := []int{80, 71, 73, 79, 69}
	if len(g.SatelliteIDs) != len(want) {
		t.Fatalf("SatelliteIDs = %v, want %v", g.SatelliteIDs, want)
	}
	for i := range want {
		if g.SatelliteIDs[i] != want[i] {
			t.Errorf("satellite %d = %d, want %d", i, g.SatelliteIDs[i], want[i])
		}
	}
	// Fields 14, 15 and 16 are PDOP, HDOP and VDOP, in that order. Reading
	// them as VDOP/HDOP/PDOP gives plausible numbers with the wrong
	// meaning.
	if !near(g.PDOP, 1.83, 1e-9) {
		t.Errorf("field 14 PDOP = %v, want 1.83", g.PDOP)
	}
	if !near(g.HDOP, 1.09, 1e-9) {
		t.Errorf("field 15 HDOP = %v, want 1.09", g.HDOP)
	}
	if !near(g.VDOP, 1.47, 1e-9) {
		t.Errorf("field 16 VDOP = %v, want 1.47", g.VDOP)
	}
	// Field 17, the system ID, is absent from this example.
	if g.HasTrailingField {
		t.Error("HasTrailingField = true for an example with no field 17")
	}
}
