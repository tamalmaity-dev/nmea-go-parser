package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

func TestAAM(t *testing.T) {
	// The standard AAM is five fields: two status flags, a radius, a unit,
	// and the waypoint name. There is no left/right field.
	a := decodeOne[AAM](t, "$GPAAM,A,A,0.10,N,WPTNME*32")

	if a.ArrivalCircle != nmea.FlagYes {
		t.Errorf("ArrivalCircle = %q, want A", a.ArrivalCircle)
	}
	if a.PerpendicularCrossed != nmea.FlagYes {
		t.Errorf("PerpendicularCrossed = %q, want A", a.PerpendicularCrossed)
	}
	if !a.HasCircleRadius || math.Abs(a.CircleRadius-0.10) > 1e-9 {
		t.Errorf("CircleRadius = %v, want 0.10", a.CircleRadius)
	}
	if a.CircleUnit != UnitNauticalMiles {
		t.Errorf("CircleUnit = %v, want nautical miles", a.CircleUnit)
	}
	if a.WaypointID != "WPTNME" {
		t.Errorf("WaypointID = %q, want WPTNME", a.WaypointID)
	}
	if !a.Arrived() {
		t.Error("Arrived = false with an A arrival flag, want true")
	}
	// 0.1 nautical miles is 185.2 m.
	if m, ok := a.CircleRadiusMetres(); !ok || math.Abs(m-185.2) > 1e-6 {
		t.Errorf("CircleRadiusMetres = %v (ok %v), want 185.2", m, ok)
	}
	// The extended form's field must stay absent for a standard sentence.
	if a.HasPerpendicularOffset {
		t.Error("HasPerpendicularOffset = true for a standard five-field AAM")
	}
}

func TestAAMExtendedForm(t *testing.T) {
	// Receivers that emit the six-field variant insert a perpendicular
	// offset and an L/R side before the radius. The L in field 3 is what
	// distinguishes it, since the standard uses field 3 for the unit letter.
	a := decodeOne[AAM](t, nmea.Frame("GPAAM,A,A,0.10,L,0.20,N,WPTNME"))
	if !a.HasPerpendicularOffset || math.Abs(a.PerpendicularOffset-0.10) > 1e-9 {
		t.Errorf("PerpendicularOffset = %v (present %v), want 0.10",
			a.PerpendicularOffset, a.HasPerpendicularOffset)
	}
	if a.Side != nmea.SideLeft {
		t.Errorf("Side = %v, want left", a.Side)
	}
	if !a.HasCircleRadius || math.Abs(a.CircleRadius-0.20) > 1e-9 {
		t.Errorf("CircleRadius = %v, want 0.20", a.CircleRadius)
	}
	if a.CircleUnit != UnitNauticalMiles {
		t.Errorf("CircleUnit = %v, want nautical miles", a.CircleUnit)
	}
	if a.WaypointID != "WPTNME" {
		t.Errorf("WaypointID = %q, want WPTNME", a.WaypointID)
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedAAM verifies that the standard form is five fields with no
// left/right flag, which several decoders get wrong by expecting six.
func TestPublishedAAM(t *testing.T) {
	// Published example, which the source prints without a leading '$'.
	a := published[AAM](t, "GPAAM,A,A,0.10,N,WPTNME")

	// Field 0: arrival circle entered. Field 1: perpendicular passed.
	if a.ArrivalCircle != nmea.FlagYes {
		t.Errorf("field 0 arrival circle = %q, want A", a.ArrivalCircle)
	}
	if a.PerpendicularCrossed != nmea.FlagYes {
		t.Errorf("field 1 perpendicular = %q, want A", a.PerpendicularCrossed)
	}
	// Field 2: the arrival circle radius.
	if !near(a.CircleRadius, 0.10, 1e-9) {
		t.Errorf("field 2 radius = %v, want 0.10", a.CircleRadius)
	}
	// Field 3: units, nautical miles. Field 4: the waypoint name.
	if a.CircleUnit != UnitNauticalMiles {
		t.Errorf("field 3 unit = %v, want nautical miles", a.CircleUnit)
	}
	if a.WaypointID != "WPTNME" {
		t.Errorf("field 4 waypoint = %q, want WPTNME", a.WaypointID)
	}
	// There is no side field in the standard form.
	if a.Side != nmea.SideUnknown {
		t.Errorf("Side = %v for a standard five-field AAM, want unknown", a.Side)
	}
	if a.HasPerpendicularOffset {
		t.Error("HasPerpendicularOffset = true for a standard five-field AAM")
	}
}
