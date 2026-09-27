package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// DBT decoder tests. The TestPublished function below checks every field
// against the example sentence in the NMEA documentation; these tests cover
// the behaviour the published example does not reach.

// TestPublishedDBT verifies the three-unit depth layout against the documented
// example.
func TestPublishedDBT(t *testing.T) {
	// Published example: 7.8 feet, 2.4 metres, 1.3 fathoms.
	d := published[DBT](t, "SDDBT,7.8,f,2.4,M,1.3,F")

	// Field 0: feet.
	if !d.HasFeet || d.Feet != 7.8 {
		t.Errorf("field 0 feet = %v (present %v), want 7.8", d.Feet, d.HasFeet)
	}
	// Field 1: metres.
	if !d.HasMetres || d.Metres != 2.4 {
		t.Errorf("field 1 metres = %v (present %v), want 2.4", d.Metres, d.HasMetres)
	}
	// Field 2: fathoms.
	if !d.HasFathoms || d.Fathoms != 1.3 {
		t.Errorf("field 2 fathoms = %v (present %v), want 1.3", d.Fathoms, d.HasFathoms)
	}
	if d.DataType() != "DBT" {
		t.Errorf("DataType = %q, want DBT", d.DataType())
	}
}

// TestDepthTriplePartialUnits covers the case the documentation calls out: a
// real sensor often reports only one of the three units. A sentence carrying
// just metres is normal, not truncated, and must still yield a depth.
func TestDepthTriplePartialUnits(t *testing.T) {
	// The documentation's own example of a partial sentence.
	metresOnly := published[DBT](t, "SDDBT,,f,22.5,M,,F")
	if metresOnly.HasFeet {
		t.Error("HasFeet = true for a blank feet field")
	}
	if metresOnly.HasFathoms {
		t.Error("HasFathoms = true for a blank fathoms field")
	}
	if got, ok := metresOnly.MetresOrConverted(); !ok || got != 22.5 {
		t.Errorf("MetresOrConverted = %v, %v; want 22.5, true", got, ok)
	}

	// Feet only, which is what a receiver configured for imperial sends.
	feetOnly := published[DBT](t, "SDDBT,7.8,f,,M,,F")
	got, ok := feetOnly.MetresOrConverted()
	if !ok {
		t.Fatal("MetresOrConverted reported nothing for a feet-only sentence")
	}
	// 7.8 feet is 2.37744 metres.
	if math.Abs(got-7.8*MetresPerFoot) > 1e-9 {
		t.Errorf("MetresOrConverted = %v, want %v", got, 7.8*MetresPerFoot)
	}

	// Fathoms only.
	fathomsOnly := published[DBT](t, "SDDBT,,f,,M,1.3,F")
	got, ok = fathomsOnly.MetresOrConverted()
	if !ok {
		t.Fatal("MetresOrConverted reported nothing for a fathoms-only sentence")
	}
	if math.Abs(got-1.3*MetresPerFathom) > 1e-9 {
		t.Errorf("MetresOrConverted = %v, want %v", got, 1.3*MetresPerFathom)
	}
}

// TestDepthTripleAllBlank covers a sounder with no reading, which is what a
// transducer out of the water reports.
func TestDepthTripleAllBlank(t *testing.T) {
	d := published[DBT](t, "SDDBT,,f,,M,,F")
	if d.HasFeet || d.HasMetres || d.HasFathoms {
		t.Error("an all-blank DBT reported a depth")
	}
	if _, ok := d.MetresOrConverted(); ok {
		t.Error("MetresOrConverted reported a depth for an all-blank sentence")
	}
	// And it must not have moved the fix.
	var f nmea.Fix
	d.ApplyFix(&f)
	if f.HasDepth {
		t.Error("an all-blank DBT set a depth on the fix")
	}
}

// TestMetresIsPreferredOverConversion checks the ordering. The receiver's own
// metre figure is used when present, because a receiver that rounds its metre
// reading differently from its foot reading should not be second-guessed.
func TestMetresIsPreferredOverConversion(t *testing.T) {
	// 7.8 feet is 2.37744 metres, but the sentence says 2.4. The sent value
	// wins, and the discrepancy between them is the receiver's rounding.
	d := published[DBT](t, "SDDBT,7.8,f,2.4,M,1.3,F")
	got, ok := d.MetresOrConverted()
	if !ok {
		t.Fatal("MetresOrConverted reported nothing")
	}
	if got != 2.4 {
		t.Errorf("MetresOrConverted = %v, want the sent 2.4 rather than the converted %v",
			got, 7.8*MetresPerFoot)
	}
}

// TestDepthUnitLettersAreNotValidated covers a deliberate choice. The unit
// letters are fixed by the format, so a receiver mislabelling one has a
// firmware bug; rejecting the sentence over that would drop a good depth
// reading on real hardware.
func TestDepthUnitLettersAreNotValidated(t *testing.T) {
	d := published[DBT](t, "SDDBT,7.8,M,2.4,f,1.3,C")
	if !d.HasFeet || d.Feet != 7.8 {
		t.Errorf("a mislabelled unit letter changed the reading: %+v", d)
	}
	if got, ok := d.MetresOrConverted(); !ok || got != 2.4 {
		t.Errorf("MetresOrConverted = %v, %v; want 2.4, true", got, ok)
	}
}

// TestDepthTruncatedRejected covers a sentence cut short. Unlike a blank unit,
// a missing field is a real framing problem.
func TestDepthTruncatedRejected(t *testing.T) {
	if err := publishedErr(t, "SDDBT,7.8"); err == nil {
		t.Error("decoding a one-field DBT succeeded, want an error")
	}
}

// TestDBTUpdatesTheFix checks that a depth is an observation and reaches the
// aggregate state.
func TestDBTUpdatesTheFix(t *testing.T) {
	var f nmea.Fix
	published[DBT](t, "SDDBT,7.8,f,2.4,M,1.3,F").ApplyFix(&f)

	if !f.HasDepth || f.DepthMetres != 2.4 {
		t.Errorf("fix depth = %v (present %v), want 2.4", f.DepthMetres, f.HasDepth)
	}
	// DBT is below the transducer, so it must not claim to be below the keel
	// or below the surface: those are different measurements.
	if f.HasKeelDepth {
		t.Error("DBT set the depth below the keel, which it does not measure")
	}
	if f.HasSurfaceDepth {
		t.Error("DBT set the depth below the surface, which it does not measure")
	}
}
