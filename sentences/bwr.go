package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// BWR is the Bearing and Distance to Waypoint, Rhumb Line sentence. Its
// layout is identical to BWC, differing only in how the receiver computed the
// distance: along a rhumb line, holding one bearing, rather than along a
// great circle.
//
//	$GPBWR,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,4.6,N,POINT
//
// The two disagree by a small amount, and the rhumb figure is larger, because
// a constant bearing is a longer path than a great circle. The difference is
// negligible over a short leg and grows on a long ocean passage, which is
// why a program that cares about total distance needs to know which the
// receiver used.
type BWR = BWC

type bwr struct{}

func (bwr) Formatter() string { return "BWR" }

func (bwr) Decode(s nmea.Sentence) (any, error) {
	out, err := decodeBearingAndDistance(s, "BWR")
	if err != nil {
		return out, err
	}
	out.Model = RhumbLine
	return out, nil
}
