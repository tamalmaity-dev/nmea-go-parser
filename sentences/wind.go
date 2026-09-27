package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// WindReading is the body shared by VWR and VWT, whose field layouts are
// identical: a wind angle relative to the bow, and the same speed in three
// units.
//
//	$IIVWR,75,R,1.0,N,0.51,M,1.85,K
//	$IIVWT,75,x,1.0,N,0.51,M,1.85,K
//
// Field layout:
//
//	0 angle, degrees from the bow, 0 to 180
//	1 side           R = starboard, L = port
//	2 speed, knots     3 N
//	4 speed, m/s       5 M
//	6 speed, km/h      7 K
//
// The two sentences differ in what the angle is measured against. VWR is
// relative wind, which is what a wind vane mounted on a moving vessel reports
// and what a sailor actually feels; VWT is true wind, corrected for the
// vessel's own motion and heading. A program wanting the wind over the water
// wants VWT, and one wanting the apparent wind wants VWR, so the distinction
// matters and neither is derived from the other here.
//
// Both are marked "not recommended for new designs" in the standard's own
// list, with MWV preferred. They are decoded because they are still on the wire
// from wind instruments built long before that ruling.
type WindReading struct {
	Base
	// AngleDegrees is the wind angle off the bow, 0 to 180. It is unsigned
	// because the side is carried separately, which is how the format does it:
	// "75 R" and "75 L" are different winds, and collapsing them to a signed
	// angle would lose which side it came from.
	AngleDegrees float64
	HasAngle     bool
	// Side is R for starboard and L for port. Some receivers send a blank or
	// an x here, which is why it is a plain string rather than an enum: a
	// wind instrument that omits the side has still reported a real angle.
	Side string

	// SpeedKnots is the stored figure, with the other two precomputed.
	SpeedKnots float64
	SpeedKmh   float64
	SpeedMps   float64
	HasSpeed   bool
}

// SideIsStarboard and SideIsPort report which side the wind is on. An
// unrecognised side reports false for both rather than guessing, because a
// reversed tack is a completely different sailing decision.
func (w WindReading) SideIsStarboard() bool { return w.Side == "R" }
func (w WindReading) SideIsPort() bool      { return w.Side == "L" }

// Tacking reports whether the wind is on either side, which is all a program
// needs in order to decide whether a tack is worth considering.
func (w WindReading) Tacking() bool { return w.SideIsPort() || w.SideIsStarboard() }

// SignedAngle returns the angle as a signed offset from the bow, positive to
// starboard. The format carries the angle unsigned with a separate side
// letter, so this is the step that turns "75 R" into a number a program can
// add to a heading.
func (w WindReading) SignedAngle() (deg float64, ok bool) {
	if !w.HasAngle {
		return 0, false
	}
	if w.SideIsPort() {
		return -w.AngleDegrees, true
	}
	return w.AngleDegrees, true
}

// TrueDirection converts a bow-relative angle to degrees true, given the
// vessel's heading. It reports false for a relative angle when the heading is
// unknown, rather than returning a number measured from the wrong datum: a
// wind direction is a compass bearing, and a bow-relative angle quietly stored
// as one is off by the vessel's heading, which is most of the day.
func (w WindReading) TrueDirection(headingDegrees float64, hasHeading bool) (deg float64, ok bool) {
	offset, ok := w.SignedAngle()
	if !ok {
		return 0, false
	}
	if !hasHeading {
		return 0, false
	}
	return normaliseBearing(offset + headingDegrees), true
}

// normaliseBearing folds a degree figure into 0 to 360, which is what every
// compass bearing in this package is expressed in. Shared by the sentences that
// add a relative angle to a heading so the wrap cannot differ between them.
func normaliseBearing(deg float64) float64 {
	for deg >= 360 {
		deg -= 360
	}
	for deg < 0 {
		deg += 360
	}
	return deg
}

// decodeWind reads the shared body.
func decodeWind(s nmea.Sentence) (WindReading, error) {
	var out WindReading
	// A sentence with no speed is a wind vane with nothing to report, which is
	// a normal state rather than a malformed sentence, so one field is enough.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}
	out.Base = newBase(s)

	var err error
	if out.AngleDegrees, out.HasAngle, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	out.Side = text(s.Field(1))
	if out.SpeedKnots, out.HasSpeed, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.HasSpeed {
		// The m/s and km/h fields are redundant with the knots figure, and a
		// receiver rounds each independently, so the knots value is the one
		// kept and the others are derived from it. Reading what the receiver
		// sent instead would leave the three fields disagreeing in the last
		// digit, with no way to tell which is the measurement.
		out.SpeedMps = knotsToMps(out.SpeedKnots)
		out.SpeedKmh = knotsToKmh(out.SpeedKnots)
	}
	return out, nil
}
