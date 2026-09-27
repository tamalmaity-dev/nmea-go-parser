package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestZDA(t *testing.T) {
	// A four-digit year and a negative local zone, both of which differ
	// from the ddmmyy form RMC uses.
	z := decodeOne[ZDA](t, "$GPZDA,160012.71,11,03,2004,-1,00*7D")

	if z.UTC.Hour != 16 || z.UTC.Minute != 0 || z.UTC.Second != 12 {
		t.Errorf("UTC = %v, want 16:00:12.710", z.UTC)
	}
	if !z.Date.Valid || z.Date.Year != 2004 || z.Date.Month != 3 || z.Date.Day != 11 {
		t.Errorf("Date = %+v, want 2004-03-11", z.Date)
	}
	if z.LocalZoneHours != -1 || z.LocalZoneMinutes != 0 {
		t.Errorf("local zone = %d:%d, want -1:00", z.LocalZoneHours, z.LocalZoneMinutes)
	}

	ts, ok := z.Timestamp()
	if !ok || ts.Year() != 2004 || ts.Month() != 3 || ts.Day() != 11 {
		t.Errorf("Timestamp = %v (ok %v), want 2004-03-11 16:00:12", ts, ok)
	}
}
func TestZDATimeOnly(t *testing.T) {
	// Some receivers send a time-only ZDA while they have no date yet. That
	// must decode, and must not invent a date.
	z := decodeOne[ZDA](t, nmea.Frame("GPZDA,160012.71"))
	if z.UTC.Hour != 16 {
		t.Errorf("UTC = %v, want 16:00:12", z.UTC)
	}
	if z.Date.Valid {
		t.Error("Date is valid for a time-only ZDA, want invalid")
	}
	if _, ok := z.Timestamp(); ok {
		t.Error("Timestamp succeeded without a date, want false")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedZDA verifies the four-digit year and the signed local zone.
func TestPublishedZDA(t *testing.T) {
	// Published example.
	z := published[ZDA](t, "GPZDA,160012.71,11,03,2004,-1,00")

	// Field 0: UTC with fractional seconds.
	if z.UTC.Hour != 16 || z.UTC.Minute != 0 || z.UTC.Second != 12 {
		t.Errorf("field 0 UTC = %v, want 16:00:12", z.UTC)
	}
	// Fields 1 to 3: day, month, four-digit year.
	if z.Date.Day != 11 || z.Date.Month != 3 || z.Date.Year != 2004 {
		t.Errorf("fields 1 to 3 = %d/%d/%d, want 11/3/2004", z.Date.Day, z.Date.Month, z.Date.Year)
	}
	// Fields 4 and 5: the local zone, negative here, which is valid and
	// must not be rejected.
	if z.LocalZoneHours != -1 || z.LocalZoneMinutes != 0 {
		t.Errorf("fields 4 and 5 = %d:%d, want -1:00", z.LocalZoneHours, z.LocalZoneMinutes)
	}
}
