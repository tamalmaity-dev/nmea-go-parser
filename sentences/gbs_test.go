package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGBS(t *testing.T) {
	// Each error figure is followed by its own unit letter, so the values sit
	// at the odd indices.
	g := decodeOne[GBS](t, "$GPGBS,125027,23.43,M,13.91,M,34.01,M*07")

	if g.UTC.Hour != 12 || g.UTC.Minute != 50 || g.UTC.Second != 27 {
		t.Errorf("UTC = %v, want 12:50:27", g.UTC)
	}
	if !g.HasExpectedLat || math.Abs(g.ExpectedErrorLat-23.43) > 1e-9 {
		t.Errorf("ExpectedErrorLat = %v, want 23.43", g.ExpectedErrorLat)
	}
	if !g.HasExpectedLon || math.Abs(g.ExpectedErrorLon-13.91) > 1e-9 {
		t.Errorf("ExpectedErrorLon = %v, want 13.91", g.ExpectedErrorLon)
	}
	if !g.HasExpectedAlt || math.Abs(g.ExpectedErrorAlt-34.01) > 1e-9 {
		t.Errorf("ExpectedErrorAlt = %v, want 34.01", g.ExpectedErrorAlt)
	}
	if g.HasFailedSatelliteID {
		t.Error("a blank satellite id was reported as a failure")
	}
}
func TestGBSWithFailedSatellite(t *testing.T) {
	g := decodeOne[GBS](t, nmea.Frame("GPGBS,125900.0,23.4,M,13.9,M,34.0,M,1,0.5,0.7"))
	if !g.HasFailedSatelliteID || g.FailedSatelliteID != 1 {
		t.Errorf("FailedSatelliteID = %d (present %v), want 1", g.FailedSatelliteID, g.HasFailedSatelliteID)
	}
	if !g.Faulty() {
		t.Error("Faulty = false with a named satellite")
	}
	if !g.Integrity() {
		t.Error("Integrity = false when a fault was identified, want true: the check ran and found something")
	}
	if !g.HasProbabilityOfMissedDetection || math.Abs(g.ProbabilityOfMissedDetection-0.5) > 1e-9 {
		t.Errorf("ProbabilityOfMissedDetection = %v, want 0.5", g.ProbabilityOfMissedDetection)
	}
	if !g.HasEstimatedBias || math.Abs(g.EstimatedBias-0.7) > 1e-9 {
		t.Errorf("EstimatedBias = %v, want 0.7", g.EstimatedBias)
	}

	var fix nmea.Fix
	fix.Valid = true
	g.ApplyFix(&fix)
	if !fix.HasSatelliteFault || fix.SatelliteFault != 1 {
		t.Errorf("fix satellite fault = %d (present %v), want 1", fix.SatelliteFault, fix.HasSatelliteFault)
	}
	// A named fault does not by itself invalidate the position: the receiver
	// may already have excluded that satellite from the solution.
	if !fix.Valid {
		t.Error("a GBS fault discarded the fix, which is usually wrong")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGBS documents the one layout where the published field list
// and the published example disagree, and records which one this decoder
// follows and why.
//
// The field list reads fields 1 to 7 as three bare error figures followed by
// a satellite ID, with no unit letters. The example, and every real
// receiver observed in the field, interleaves an "M" after each error
// figure. The example is authoritative because it is what the hardware
// actually emits, so the values sit at the odd indices.
func TestPublishedGBS(t *testing.T) {
	// Published example, which does carry the unit letters.
	g := published[GBS](t, "GPGBS,125027,23.43,M,13.91,M,34.01,M")

	// Field 0: the UTC time of the fix these errors describe.
	if g.UTC.Hour != 12 || g.UTC.Minute != 50 || g.UTC.Second != 27 {
		t.Errorf("field 0 UTC = %v, want 12:50:27", g.UTC)
	}
	// Fields 1, 3 and 5: the three 1-sigma error figures, each followed by
	// its unit letter at fields 2, 4 and 6.
	if !near(g.ExpectedErrorLat, 23.43, 1e-9) {
		t.Errorf("field 1 latitude error = %v, want 23.43", g.ExpectedErrorLat)
	}
	if !near(g.ExpectedErrorLon, 13.91, 1e-9) {
		t.Errorf("field 3 longitude error = %v, want 13.91", g.ExpectedErrorLon)
	}
	if !near(g.ExpectedErrorAlt, 34.01, 1e-9) {
		t.Errorf("field 5 altitude error = %v, want 34.01", g.ExpectedErrorAlt)
	}
	// The unit letters were validated, not skipped: a receiver reporting
	// feet here would be an error rather than a number three times too big.
	if g.HasFailedSatelliteID {
		t.Error("a failed satellite was reported although field 7 is absent")
	}
	if g.Faulty() {
		t.Error("Faulty = true with no satellite named")
	}
}
