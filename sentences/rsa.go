package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// RSA is the Rudder Sensor Angle sentence. A vessel with two rudders reports
// both, and the difference between them is the yaw rate, which is how a
// program derives the rate of turn when no ROT sentence is available.
//
//	$IIRSA,10.5,A,,V
//	$IIRSA,10.5,A,-3.2,A
//
// Field layout:
//
//	0 starboard rudder angle  degrees, or the single rudder on one-engine boats
//	1 status                 A valid, V invalid
//	2 port rudder angle      degrees
//	3 status                 A valid, V invalid
//
// The sign convention is that a negative angle turns to port. A single-rudder
// vessel leaves the port fields blank, and a program that required both would
// miss every such boat.
type RSA struct {
	Base
	// Starboard is the starboard or single rudder angle in degrees, negative
	// to port.
	Starboard    float64
	HasStarboard bool
	// Port is the port rudder angle in degrees.
	Port    float64
	HasPort bool
	// StarboardStatus and PortStatus are A when the reading is valid.
	StarboardStatus nmea.StatusFlag
	PortStatus      nmea.StatusFlag
}

// RudderAngle returns the angle that best represents the vessel's steering.
// On a single-rudder vessel that is the starboard field alone; with two
// rudders the mean is the heading-deflecting angle, and the half-difference
// is the yaw the hull is actually experiencing.
func (r *RSA) RudderAngle() (mean, yaw float64, ok bool) {
	switch {
	case r.HasStarboard && r.HasPort:
		return (r.Starboard + r.Port) / 2, (r.Starboard - r.Port) / 2, true
	case r.HasStarboard:
		return r.Starboard, 0, true
	default:
		return 0, 0, false
	}
}

// RateOfTurnDegPerMin estimates the rate of turn from the rudder angle,
// which is a rough figure useful only when no ROT sentence exists. The
// constant is a nominal turning circle for a small vessel and is not
// calibrated; a real rate needs a ROT or a heading series.
func (r *RSA) RateOfTurnDegPerMin() (float64, bool) {
	angle, _, ok := r.RudderAngle()
	if !ok {
		return 0, false
	}
	// A full rudder of 35 degrees is taken to produce about 30 degrees per
	// minute of turn in a small boat, and the response is linear below that.
	const fullRudder = 35.0
	const degPerMinAtFull = 30.0
	return angle / fullRudder * degPerMinAtFull, true
}

type rsa struct{}

func (rsa) Formatter() string { return "RSA" }

func (rsa) Decode(s nmea.Sentence) (any, error) {
	// A rudder angle sensor that is not fitted leaves the whole sentence
	// blank, so an empty field list is the only failure.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := RSA{Base: newBase(s)}

	var err error
	if out.Starboard, out.HasStarboard, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	out.StarboardStatus = nmea.ParseStatusFlag(s.Field(1))
	if out.Port, out.HasPort, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	out.PortStatus = nmea.ParseStatusFlag(s.Field(3))
	return out, nil
}

// ApplyFix folds the RSA into the fix state as the rudder angle.
func (r RSA) ApplyFix(f *nmea.Fix) {
	if angle, _, ok := r.RudderAngle(); ok {
		f.RudderAngle, f.HasRudder = angle, true
	}
}
