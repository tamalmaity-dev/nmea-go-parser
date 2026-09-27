package sentences

import (
	"testing"
)

// DTM published-example checks: the field layout is verified against
// the example sentence in the NMEA documentation.

// TestPublishedDTM verifies the datum fields, whose offsets are in arc
// minutes and signed by the direction fields.
func TestPublishedDTM(t *testing.T) {
	// The published example is the short two-field form, which is what
	// receivers actually send.
	d := published[DTM](t, "GPDTM,W84,C")
	if d.LocalDatum != "W84" {
		t.Errorf("field 0 datum = %q, want W84", d.LocalDatum)
	}
	if d.Subcode != "C" {
		t.Errorf("field 1 subcode = %q, want C", d.Subcode)
	}
	// Fields 2 to 7 are absent, so the offsets must be flagged absent rather
	// than read as zero.
	if d.HasLatOffset || d.HasLonOffset || d.HasAltOffset {
		t.Error("absent offset fields were reported as present")
	}

	// The full form, to check the field order and the sign convention.
	full := published[DTM](t, "GPDTM,W84,,0.5,S,1.5,W,0.0,W84")
	if !near(full.LatOffset, -0.5, 1e-9) {
		t.Errorf("field 2 latitude offset = %v, want -0.5 for a south offset", full.LatOffset)
	}
	if !near(full.LonOffset, -1.5, 1e-9) {
		t.Errorf("field 4 longitude offset = %v, want -1.5 for a west offset", full.LonOffset)
	}
	if full.DatumName != "W84" {
		t.Errorf("field 7 datum name = %q, want W84", full.DatumName)
	}
}
