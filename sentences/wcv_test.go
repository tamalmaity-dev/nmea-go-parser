package sentences

import (
	"testing"
)

// TestPublishedWCV verifies the waypoint closure velocity layout against the
// documented field list:
//
//	$--WCV,x.x,N,c--c,a
//
// gpsd publishes no worked example, so the values come from the field list and
// from a real receiver sentence:
//
//	$GPWCV,2.3,N,DEST*1E
func TestPublishedWCV(t *testing.T) {
	w := published[WCV](t, "GPWCV,2.3,N,DEST")

	// Field 0: the rate at which the vessel is closing on the waypoint.
	if !w.HasVelocity || !near(w.Velocity, 2.3, 1e-9) {
		t.Errorf("field 0 velocity = %v (present %v), want 2.3", w.Velocity, w.HasVelocity)
	}
	// Field 1: knots, the only defined unit.
	if w.Unit != SpeedUnitKnots {
		t.Errorf("field 1 unit = %v, want knots", w.Unit)
	}
	// Field 2: the waypoint the velocity applies to.
	if w.WaypointID != "DEST" {
		t.Errorf("field 2 waypoint = %q, want DEST", w.WaypointID)
	}
	// Field 3 is the FAA mode indicator, absent from this sentence.
	if w.Mode != "" {
		t.Errorf("field 3 mode = %q, want empty", w.Mode)
	}

	// A positive velocity means the vessel is approaching.
	closing, ok := w.Closing()
	if !ok || !closing {
		t.Errorf("Closing = %v, %v; want true, true", closing, ok)
	}
}

// TestWCVNegativeIsOpening checks that a negative closure velocity is reported
// as opening rather than closing. The value is signed precisely so a program
// can tell the two apart, so a decoder that takes the magnitude loses that.
func TestWCVNegativeIsOpening(t *testing.T) {
	w := published[WCV](t, "GPWCV,-2.3,N,DEST")

	if !near(w.Velocity, -2.3, 1e-9) {
		t.Errorf("velocity = %v, want -2.3", w.Velocity)
	}
	closing, ok := w.Closing()
	if !ok {
		t.Fatal("Closing reported no value for a sentence carrying a velocity")
	}
	if closing {
		t.Error("a negative closure velocity reports as closing")
	}
}

// TestWCVNoActiveWaypoint covers the receiver with no active waypoint, which
// reports a zero velocity. Zero is a real measurement here rather than a
// missing one, but it is not a direction, so Closing reports no value.
func TestWCVNoActiveWaypoint(t *testing.T) {
	w := published[WCV](t, "GPWCV,0.0,N,")

	if !w.HasVelocity {
		t.Error("HasVelocity = false for an explicit zero velocity")
	}
	if _, ok := w.Closing(); ok {
		t.Error("Closing reported a direction for a zero velocity")
	}
}

// TestWCVRejectsMislabeledUnit checks the unit reference. N is the only defined
// unit, so a value carrying K is either a different sentence or a receiver
// that is not following the format.
func TestWCVRejectsMislabeledUnit(t *testing.T) {
	if err := publishedErr(t, "GPWCV,4.0,K,DEST"); err == nil {
		t.Error("decoding a WCV velocity labelled K succeeded, want an error")
	}
}
