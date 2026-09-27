package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// VWR decoder tests. The TestPublished function below checks every field
// against the example sentence in the NMEA documentation; these tests cover
// the behaviour the published example does not reach.

// TestPublishedVWR verifies the relative wind layout against the documented
// example: angle, side, then the speed in three units.
func TestPublishedVWR(t *testing.T) {
	// Published example: 75 degrees to starboard, 1.0 knots.
	w := published[VWR](t, "IIVWR,75,R,1.0,N,0.51,M,1.85,K")

	// Field 0: the angle off the bow, unsigned, 0 to 180.
	if !w.HasAngle || w.AngleDegrees != 75 {
		t.Errorf("field 0 angle = %v (present %v), want 75", w.AngleDegrees, w.HasAngle)
	}
	// Field 1: the side. This is what makes "75 R" and "75 L" different winds.
	if w.Side != "R" {
		t.Errorf("field 1 side = %q, want R", w.Side)
	}
	if !w.SideIsStarboard() {
		t.Error("SideIsStarboard = false for an R side")
	}
	if w.SideIsPort() {
		t.Error("SideIsPort = true for an R side")
	}
	// Field 2: the speed in knots, which is the stored figure.
	if !w.HasSpeed || w.SpeedKnots != 1.0 {
		t.Errorf("field 2 knots = %v (present %v), want 1.0", w.SpeedKnots, w.HasSpeed)
	}
	// The other two units are derived rather than read, because a receiver
	// rounds each independently and reading all three leaves them disagreeing
	// in the last digit. 1.0 knots is 0.5144 m/s and 1.852 km/h.
	if math.Abs(w.SpeedMps-0.51) > 0.01 {
		t.Errorf("SpeedMps = %v, want about 0.51", w.SpeedMps)
	}
	if math.Abs(w.SpeedKmh-1.85) > 0.01 {
		t.Errorf("SpeedKmh = %v, want about 1.85", w.SpeedKmh)
	}
}

// TestVWRSpeedOnly covers a wind vane reporting an angle with no speed, which is
// what a failed anemometer looks like.
func TestVWRSpeedOnly(t *testing.T) {
	w := published[VWR](t, "IIVWR,024,L,,N,,M,,K")

	if !w.HasAngle || w.AngleDegrees != 24 {
		t.Errorf("angle = %v (present %v), want 24", w.AngleDegrees, w.HasAngle)
	}
	if w.Side != "L" || !w.SideIsPort() {
		t.Errorf("side = %q, want L", w.Side)
	}
	if w.HasSpeed {
		t.Error("HasSpeed = true for a blank speed")
	}
	if w.SpeedMps != 0 || w.SpeedKmh != 0 {
		t.Errorf("derived speeds = %v and %v, want zero when there is no reading",
			w.SpeedMps, w.SpeedKmh)
	}
}

// TestVWRAllBlank covers a wind instrument that is powered but reporting
// nothing, which several send as an all-blank sentence rather than staying
// quiet.
func TestVWRAllBlank(t *testing.T) {
	w := published[VWR](t, "IIVWR,,,,,,,,")

	if w.HasAngle || w.HasSpeed {
		t.Error("an all-blank VWR reported a reading")
	}
	if w.Tacking() {
		t.Error("Tacking = true with no side reported")
	}
	// And it must not have moved the fix.
	var f nmea.Fix
	w.ApplyFix(&f)
	if f.HasWindSpeed || f.HasWindDirection {
		t.Error("an all-blank VWR set wind data on the fix")
	}
}

// TestVWRUnknownSideIsNotGuessed covers a receiver that sends a side letter
// this library does not know, or none at all. A reversed tack is a completely
// different sailing decision, so the side is reported as-is rather than assumed.
func TestVWRUnknownSideIsNotGuessed(t *testing.T) {
	for _, side := range []string{"x", "", "?"} {
		w := published[VWR](t, "IIVWR,75,"+side+",1.0,N,0.51,M,1.85,K")
		if w.Side != side {
			t.Errorf("side = %q, want %q passed through", w.Side, side)
		}
		if w.SideIsPort() || w.SideIsStarboard() {
			t.Errorf("side %q reported as a known side, want neither", side)
		}
		if w.Tacking() {
			t.Errorf("Tacking = true for the unrecognised side %q", side)
		}
	}
}

// TestVWRTrueDirectionNeedsHeading is the important one. A bow-relative angle
// is not a compass bearing: storing it as one puts the wind direction out by the
// vessel's heading, which is most of the day.
func TestVWRTrueDirectionNeedsHeading(t *testing.T) {
	// Heading 090, wind 75 to starboard: the wind is at 165.
	w := published[VWR](t, "IIVWR,75,R,1.0,N,0.51,M,1.85,K")

	var f nmea.Fix
	w.ApplyFix(&f)
	if f.HasWindDirection {
		t.Error("a bow-relative wind angle set a compass direction with no heading known")
	}

	f.HeadingDegrees, f.HasHeading = 90, true
	w.ApplyFix(&f)
	if !f.HasWindDirection {
		t.Fatal("no wind direction after a heading was known")
	}
	if math.Abs(f.WindDirection-165) > 1e-9 {
		t.Errorf("WindDirection = %v, want 165: heading 090 plus 75 to starboard", f.WindDirection)
	}

	// To port, the angle subtracts, and the result wraps rather than going
	// negative.
	port := published[VWR](t, "IIVWR,75,L,1.0,N,0.51,M,1.85,K")
	f2 := nmea.Fix{HeadingDegrees: 90, HasHeading: true}
	port.ApplyFix(&f2)
	if math.Abs(f2.WindDirection-15) > 1e-9 {
		t.Errorf("WindDirection = %v, want 15: heading 090 minus 75 to port", f2.WindDirection)
	}

	// A bow-relative angle just past zero to port from a heading of 10 wraps
	// through north rather than reporting a negative bearing.
	wrap := published[VWR](t, "IIVWR,30,L,1.0,N,0.51,M,1.85,K")
	f3 := nmea.Fix{HeadingDegrees: 10, HasHeading: true}
	wrap.ApplyFix(&f3)
	if f3.WindDirection < 0 || f3.WindDirection >= 360 {
		t.Errorf("WindDirection = %v, want it wrapped into 0..360", f3.WindDirection)
	}
	if math.Abs(f3.WindDirection-340) > 1e-9 {
		t.Errorf("WindDirection = %v, want 340", f3.WindDirection)
	}
}

// TestVWRSignedAngle covers the conversion the direction depends on.
func TestVWRSignedAngle(t *testing.T) {
	for _, tc := range []struct {
		side string
		want float64
	}{
		{"R", 75},
		{"L", -75},
		{"x", 75}, // unknown side treated as starboard for the offset only
	} {
		w := published[VWR](t, "IIVWR,75,"+tc.side+",1.0,N,0.51,M,1.85,K")
		got, ok := w.SignedAngle()
		if !ok {
			t.Errorf("side %q: SignedAngle reported no value", tc.side)
			continue
		}
		if got != tc.want {
			t.Errorf("side %q: SignedAngle = %v, want %v", tc.side, got, tc.want)
		}
	}
}
