package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// BOD is the Bearing, Origin to Destination sentence. It reports a bearing
// between two named points and is normally emitted once per leg rather than
// continuously, so it suits a log of the course set rather than a live
// display.
//
//	$GPBOD,045.,T,023.,M,Destination,ARRVL*4E
//	$GPBOD,099.3,T,105.6,M,POINTB*01
//
// Field layout, per the standard:
//
//	0 bearing true      degrees
//	1 reference         T for true
//	2 bearing magnetic  degrees
//	3 reference         M for magnetic
//	4 destination       waypoint name at the far end
//	5 origin            waypoint name at the near end, optional
//
// The origin is optional in practice: a receiver in goto mode has only a
// destination, and the second example above is a complete, valid sentence
// without one. It is therefore optional here too.
//
// The reference fields stop a magnetic bearing being read as a true one, so
// they are validated rather than ignored.
type BOD struct {
	Base
	// BearingTrue and BearingMagnetic are the two bearings, each with a
	// presence flag. Only one is normally populated.
	BearingTrue     float64
	HasBearingTrue  bool
	BearingMagnetic float64
	HasBearingMag   bool
	// Destination is the far waypoint and Origin the near one. Origin is
	// empty when the receiver reported only a destination.
	Destination string
	Origin      string
}

type bod struct{}

func (bod) Formatter() string { return "BOD" }

func (bod) Decode(s nmea.Sentence) (any, error) {
	// The destination name is what makes the sentence useful, so it is the
	// one field that is required.
	if !s.HasFields(5) {
		return nil, needFields(s, 5)
	}
	out := BOD{Base: newBase(s)}

	var err error
	if out.BearingTrue, out.HasBearingTrue, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.HasBearingTrue && !referenceOK(s.Field(1), "T") {
		return out, errMislabeled("BOD", "true", s.Field(1))
	}
	if out.BearingMagnetic, out.HasBearingMag, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.HasBearingMag && !referenceOK(s.Field(3), "M") {
		return out, errMislabeled("BOD", "magnetic", s.Field(3))
	}

	out.Destination = text(s.Field(4))
	out.Origin = text(s.Field(5))
	return out, nil
}
