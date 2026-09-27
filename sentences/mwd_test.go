package sentences

import (
	"testing"
)

// MWD published-example checks: the field layout is verified against
// the example sentence in the NMEA documentation.

// TestPublishedMWD verifies the doubled value and reference pairs.
func TestPublishedMWD(t *testing.T) {
	// Published example.
	m := published[MWD](t, "WIMWD,302.4,T,289.6,M,10.5,N,5.4,M")

	// Field 0: true wind direction. Field 1: its T reference.
	if !near(m.DirectionTrue, 302.4, 1e-9) {
		t.Errorf("field 0 true direction = %v, want 302.4", m.DirectionTrue)
	}
	// Field 2: magnetic wind direction. Field 3: its M reference.
	if !near(m.DirectionMagnetic, 289.6, 1e-9) {
		t.Errorf("field 2 magnetic direction = %v, want 289.6", m.DirectionMagnetic)
	}
	// Field 4: speed. Field 5: the unit, N for knots. Field 6: the same
	// speed in a second unit. Field 7: that unit, M for m/s.
	if !near(m.SpeedKnots, 10.5, 1e-9) {
		t.Errorf("field 4 speed = %v, want 10.5", m.SpeedKnots)
	}
	if m.SpeedUnit != SpeedUnitKnots {
		t.Errorf("field 5 unit = %v, want knots", m.SpeedUnit)
	}
	if !near(m.SpeedKnotsAlt, 5.4, 1e-9) {
		t.Errorf("field 6 speed = %v, want 5.4", m.SpeedKnotsAlt)
	}
	if m.SpeedUnitAlt != SpeedUnitMps {
		t.Errorf("field 7 unit = %v, want m/s", m.SpeedUnitAlt)
	}
	// The first pair is normalised to knots, so the two unit pairs can be
	// compared directly: 5.4 m/s is 5.4 * 1.94384 knots, which rounds to the
	// 10.5 the example prints.
	if !near(m.SpeedKnots, 5.4*SpeedUnitMps.ToKnots(1.0), 1e-2) {
		t.Errorf("the two unit pairs disagree: %v knots against %v", m.SpeedKnots, m.SpeedKnotsAlt)
	}
}
