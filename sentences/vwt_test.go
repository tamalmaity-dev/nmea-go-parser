package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// VWT decoder tests. VWT shares its layout with VWR byte for byte, so these
// cover what is specific to it rather than repeating the field checks.

// TestPublishedVWT verifies the true wind layout against the documented
// example.
func TestPublishedVWT(t *testing.T) {
	w := published[VWT](t, "IIVWT,75,x,1.0,N,0.51,M,1.85,K")

	if !w.HasAngle || w.AngleDegrees != 75 {
		t.Errorf("field 0 angle = %v (present %v), want 75", w.AngleDegrees, w.HasAngle)
	}
	if w.Side != "x" {
		t.Errorf("field 1 side = %q, want x passed through", w.Side)
	}
	if !w.HasSpeed || w.SpeedKnots != 1.0 {
		t.Errorf("field 2 knots = %v (present %v), want 1.0", w.SpeedKnots, w.HasSpeed)
	}
	if math.Abs(w.SpeedMps-0.51) > 0.01 {
		t.Errorf("SpeedMps = %v, want about 0.51", w.SpeedMps)
	}
	if w.DataType() != "VWT" {
		t.Errorf("DataType = %q, want VWT", w.DataType())
	}
}

// TestVWTPortSide verifies the port side, which is the case that distinguishes
// the two sentences' sign handling from a plain copy of VWR's test.
func TestVWTPortSide(t *testing.T) {
	w := published[VWT](t, "IIVWT,75,L,1.0,N,0.51,M,1.85,K")
	if !w.SideIsPort() {
		t.Error("SideIsPort = false for an L side")
	}
	got, ok := w.SignedAngle()
	if !ok || got != -75 {
		t.Errorf("SignedAngle = %v, %v; want -75, true", got, ok)
	}
}

// TestVWTUpdatesTheFix checks that the true wind reaches the fix. It lands in
// the same speed field as every other wind sentence, which is a deliberate
// choice: the fix answers "what is the wind doing", and a program that needs
// to know whether a figure is apparent or true reads the sentence instead.
func TestVWTUpdatesTheFix(t *testing.T) {
	var f nmea.Fix
	published[VWT](t, "IIVWT,75,R,12.5,N,6.43,M,23.15,K").ApplyFix(&f)

	if !f.HasWindSpeed || f.WindSpeedKnots != 12.5 {
		t.Errorf("fix wind speed = %v (present %v), want 12.5", f.WindSpeedKnots, f.HasWindSpeed)
	}
	// VWT's angle is bow-relative like VWR's, so without a heading it is not a
	// compass direction and must not become one.
	if f.HasWindDirection {
		t.Error("VWT set a compass wind direction with no heading known")
	}
}

// TestVWTDoesNotDeriveFromVWR records the limitation honestly. The receiver does
// the apparent-to-true correction using its own heading and speed, so a program
// cannot reconstruct VWT from VWR alone. This test pins that, because a future
// change that tried to derive one from the other would be wrong.
func TestVWTDoesNotDeriveFromVWR(t *testing.T) {
	// The same reading in both sentences is the receiver reporting the same
	// wind, not evidence that one was computed from the other.
	relative := published[VWR](t, "IIVWR,75,R,12.5,N,6.43,M,23.15,K")
	true_ := published[VWT](t, "IIVWT,75,R,12.5,N,6.43,M,23.15,K")

	if relative.SpeedKnots != true_.SpeedKnots {
		t.Errorf("the two sentences disagree on a reading they should share: %v and %v",
			relative.SpeedKnots, true_.SpeedKnots)
	}
	// Each is its own decoded value: nothing links them.
	if relative.DataType() == true_.DataType() {
		t.Error("VWR and VWT decoded to the same type, so the distinction is lost")
	}
}
