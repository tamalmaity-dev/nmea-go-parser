package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// AAM is the Waypoint Arrival Alarm sentence. It reports that the vessel has
// reached the current waypoint, and is emitted once per arrival rather than
// continuously, so it is an event rather than a state.

//------------------------------------------------------------------------------------------
//	$GPAAM,A,A,0.10,N,WPTNME*43
//------------------------------------------------------------------------------------------

// Field layout, per the standard:
//
//	0 arrival circle   A entered, V not entered
//	1 perpendicular     A crossed at the waypoint, V not crossed
//	2 circle radius     in the unit given by field 3
//	3 radius unit       N nautical miles
//	4 waypoint id       the waypoint that was reached
//
// There is no left/right field. Several receivers emit a six-field variant
// with a perpendicular offset and side inserted before the unit, so
// PerpendicularOffset is populated when a six-field sentence arrives and
// left zero for the standard five-field form.

type AAM struct {
	Base
	// ArrivalCircle is A once the vessel is inside the arrival radius. This
	// is the flag to watch for a one-shot arrival event.
	ArrivalCircle nmea.StatusFlag
	// PerpendicularCrossed is A once the vessel has crossed the centre line
	// since the last waypoint.
	PerpendicularCrossed nmea.StatusFlag
	CircleRadius         float64 // CircleRadius is the arrival radius, in CircleUnit.
	HasCircleRadius      bool
	CircleUnit           DistanceUnit
	// WaypointID is the waypoint that was reached.
	WaypointID string

	// PerpendicularOffset is the distance from the centre line, populated
	// only by the six-field variant. Use HasPerpendicularOffset to tell an
	// absent field from a genuine zero.
	PerpendicularOffset    float64
	HasPerpendicularOffset bool
	// Side is which side of the centre line the vessel passed, from the same
	// variant.
	Side nmea.Side
}

// StandardAAMFields is the number of payload fields in the standard form.
const StandardAAMFields = 5

type aam struct{}

func (aam) Formatter() string { return "AAM" }

func (aam) Decode(s nmea.Sentence) (any, error) {
	// The waypoint id closes the sentence and identifies what was reached,
	// so the standard field count is the minimum.
	if !s.HasFields(StandardAAMFields) {
		return nil, needFields(s, StandardAAMFields)
	}
	out := AAM{Base: newBase(s)}

	out.ArrivalCircle = nmea.ParseStatusFlag(s.Field(0))
	out.PerpendicularCrossed = nmea.ParseStatusFlag(s.Field(1))

	// The standard puts the radius in field 2 and its unit in field 3. The
	// common six-field variant inserts a perpendicular offset and side, which
	// shifts the radius to field 3. The two forms are told apart by whether
	// the field after the perpendicular offset is a unit letter.
	if hasAAMExtension(s) {
		offset, present, err := optionalFloat(s, 2)
		if err != nil {
			return out, err
		}
		out.PerpendicularOffset, out.HasPerpendicularOffset = offset, present
		if out.Side, err = nmea.ParseSide(s.Field(3)); err != nil {
			return out, err
		}
		if out.CircleRadius, out.HasCircleRadius, err = optionalFloat(s, 4); err != nil {
			return out, err
		}
		if out.CircleUnit, err = ParseDistanceUnit(s.Field(5)); err != nil {
			return out, err
		}
		out.WaypointID = text(s.Field(6))
		return out, nil
	}

	var err error
	if out.CircleRadius, out.HasCircleRadius, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.CircleUnit, err = ParseDistanceUnit(s.Field(3)); err != nil {
		return out, err
	}
	out.WaypointID = text(s.Field(4))
	return out, nil
}

// hasAAMExtension distinguishes the six-field variant from the standard
// five-field form.
//
// In the standard form field 3 is the radius unit, a bare N or K. In the
// extended form field 3 is a side, a bare L or R. Testing for L or R is
// unambiguous because the two flag alphabets do not overlap.
func hasAAMExtension(s nmea.Sentence) bool {
	switch s.Field(3) {
	case "L", "l", "R", "r":
		return true
	default:
		return false
	}
}

// Arrived reports whether the vessel has entered the arrival circle.
func (a AAM) Arrived() bool { return a.ArrivalCircle.Valid() }

// CircleRadiusMetres returns the arrival radius in metres, converted from
// whichever unit the receiver used.
func (a *AAM) CircleRadiusMetres() (metres float64, ok bool) {
	if !a.HasCircleRadius {
		return 0, false
	}
	return a.CircleUnit.ToMetres(a.CircleRadius), true
}

// PerpendicularMetres returns the distance from the centre line in metres.
// It is only meaningful for the six-field variant.
func (a *AAM) PerpendicularMetres() (metres float64, ok bool) {
	if !a.HasPerpendicularOffset {
		return 0, false
	}
	return a.CircleUnit.ToMetres(a.PerpendicularOffset), true
}
