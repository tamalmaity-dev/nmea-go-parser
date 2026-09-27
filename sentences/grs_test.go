package sentences

import (
	"testing"
)

// GRS published-example checks: the field layout is verified against
// the example sentence in the NMEA documentation.

// TestPublishedGRS verifies that the residuals are positional and start at
// field 2, with the system and signal IDs at 14 and 15.
func TestPublishedGRS(t *testing.T) {
	// Published example.
	g := published[GRS](t, "GPGRS,024603.00,1,-1.8,-2.7,0.3,,,,,,,")

	// Field 0: the time of the associated fix.
	if g.UTC.Hour != 2 || g.UTC.Minute != 46 || g.UTC.Second != 3 {
		t.Errorf("field 0 UTC = %v, want 02:46:03", g.UTC)
	}
	// Field 1: mode 1, residuals calculated after the GGA.
	if g.Mode != 1 {
		t.Errorf("field 1 mode = %d, want 1", g.Mode)
	}
	// Fields 2 to 4: residuals for satellites one, two and three. They are
	// positional, so their order matters and there is no satellite number to
	// key them by.
	want := []float64{-1.8, -2.7, 0.3}
	if len(g.Residuals) != len(want) {
		t.Fatalf("Residuals = %v, want %v", g.Residuals, want)
	}
	for i := range want {
		if !near(g.Residuals[i], want[i], 1e-9) {
			t.Errorf("residual %d = %v, want %v", i, g.Residuals[i], want[i])
		}
	}
	if len(g.SatelliteNumbers) != 3 ||
		g.SatelliteNumbers[0] != 1 || g.SatelliteNumbers[2] != 3 {
		t.Errorf("SatelliteNumbers = %v, want 1, 2, 3", g.SatelliteNumbers)
	}
	// The remaining slots are blank, not zero.
	if len(g.Residuals) != 3 {
		t.Errorf("blank slots were read as residuals: %v", g.Residuals)
	}
}
