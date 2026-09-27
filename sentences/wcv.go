package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// WCV is the Waypoint Closure Velocity sentence. It reports how fast the
// vessel is closing on the active waypoint, which is the figure a program
// needs to decide when to advance to the next waypoint: a fixed time or
// distance threshold is worse than using the actual closure rate.
//
//	$GPWCV,2.3,N,DEST
//
// Field layout:
//
//	0 velocity     knots, positive when closing
//	1 units        N knots
//	2 waypoint id
//	3 FAA mode     NMEA 3.0 and later, documented as never null
//
// The sign is the useful part. A negative velocity means the vessel is
// opening the distance, usually because it has passed the waypoint without
// noticing, and a program that ignored the sign would conclude the vessel was
// still approaching.
type WCV struct {
	Base
	// Velocity is knots, positive when closing on the waypoint.
	Velocity    float64
	HasVelocity bool
	// Unit is N for knots. The sentence has no other defined unit.
	Unit SpeedUnit
	// WaypointID is the waypoint the velocity applies to.
	WaypointID string
	// Mode is the FAA mode indicator, NMEA 3.0 and later.
	Mode string
}

// Closing reports whether the vessel is approaching the waypoint, and false
// when the velocity is unknown or effectively zero.
func (w WCV) Closing() (closing bool, ok bool) {
	if !w.HasVelocity || w.Velocity == 0 {
		return false, false
	}
	return w.Velocity > 0, true
}

type wcv struct{}

func (wcv) Formatter() string { return "WCV" }

func (wcv) Decode(s nmea.Sentence) (any, error) {
	// A WCV with no velocity is a receiver with no active waypoint, which is
	// a normal state, so only the speed field itself is optional.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := WCV{Base: newBase(s)}

	var err error
	if out.Velocity, out.HasVelocity, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HasVelocity && !referenceOK(s.Field(1), "N") {
		return out, errMislabeled("WCV", "closure velocity", s.Field(1))
	}
	out.Unit = SpeedUnitKnots
	out.WaypointID = text(s.Field(2))
	out.Mode = text(s.Field(3))
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: the FAA mode arrived in
// NMEA 3.0.
func (w WCV) NMEASupports() []nmea.Feature {
	if w.Mode != "" {
		return nmea.FeatureOnly(nmea.FeatureFAA)
	}
	return nil
}
