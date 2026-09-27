package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VWR is the Relative Wind Speed and Angle sentence: what a wind vane mounted
// on a moving vessel reports, which is the apparent wind a crew actually feels.
//
//	$IIVWR,75,R,1.0,N,0.51,M,1.85,K
//	$IIVWR,024,L,018,N,,,,*5E
//	$IIVWR,,,,,,,,*53
//
// Field layout:
//
//	0 angle, degrees from the bow, 0 to 180
//	1 side           R = starboard, L = port
//	2 speed, knots     3 N
//	4 speed, m/s       5 M
//	6 speed, km/h      7 K
//
// VWR is marked "not recommended for new designs" in the standard's own list,
// with MWV preferred. It is decoded because wind instruments built long before
// that ruling still send it, and because a program that cannot read VWR cannot
// read a great many sailboats.
//
// The three speed fields are redundant. The knots value is kept and the other
// two derived, because a receiver rounds each independently and reading all
// three would leave them disagreeing in the last digit.
type VWR struct {
	WindReading
}

type vwr struct{}

func (vwr) Formatter() string { return "VWR" }

func (vwr) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeWind(s)
	if err != nil {
		return body, err
	}
	return VWR{WindReading: body}, nil
}

// ApplyFix folds the apparent wind into the fix.
//
// The wind speed lands in the same field as every other wind sentence, so a
// receiver emitting both MWV and VWR leaves whichever arrived last. That is a
// deliberate choice over adding a second field: the aggregate fix is for "what
// is the wind doing", and a program that needs to know whether a figure is the
// apparent or the true wind should read the sentence, where the distinction is
// explicit. The apparent wind is not stored as if it were the true wind.
func (v VWR) ApplyFix(f *nmea.Fix) {
	if v.HasSpeed {
		f.WindSpeedKnots, f.HasWindSpeed = v.SpeedKnots, true
	}
	// The angle is relative to the bow, so it becomes a compass direction only
	// when the vessel's heading is known.
	if deg, ok := v.TrueDirection(f.HeadingDegrees, f.HasHeading); ok {
		f.WindDirection, f.HasWindDirection = deg, true
	}
}
