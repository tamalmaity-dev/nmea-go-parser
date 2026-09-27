package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VWT is the True Wind Speed and Angle sentence: the wind corrected for the
// vessel's own motion, which is the wind over the water rather than the wind
// felt on deck.
//
//	$IIVWT,75,R,1.0,N,0.51,M,1.85,K
//
// Field layout:
//
//	0 angle, degrees from the bow, 0 to 180
//	1 side           R = starboard, L = port
//	2 speed, knots     3 N
//	4 speed, m/s       5 M
//	6 speed, km/h      7 K
//
// VWT shares its layout with VWR byte for byte and differs only in what the
// angle is measured against. The receiver does the correction, using its own
// heading and speed, so a program cannot derive VWT from VWR alone: it would
// need the vessel's motion, which is exactly what the receiver already had.
//
// Like VWR it is on the standard's "not recommended for new designs" list.
type VWT struct {
	WindReading
}

type vwt struct{}

func (vwt) Formatter() string { return "VWT" }

func (vwt) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeWind(s)
	if err != nil {
		return body, err
	}
	return VWT{WindReading: body}, nil
}

// ApplyFix folds the true wind into the fix, keeping it apart from the apparent
// wind that VWR reports. Storing both in one field would make the pair
// disagree depending on which arrived last.
func (v VWT) ApplyFix(f *nmea.Fix) {
	if v.HasSpeed {
		f.WindSpeedKnots, f.HasWindSpeed = v.SpeedKnots, true
	}
}
