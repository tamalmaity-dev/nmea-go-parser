package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// GreatCircle is how BWC computes its distance. BWR, which follows the same
// rhumb line, is the constant-bearing alternative; between them a program can
// tell which model a receiver used.
const GreatCircle = "great circle"

// RhumbLine is the constant-bearing model, carried by BWR.
const RhumbLine = "rhumb line"

// BWC is the Bearing and Distance to Waypoint, Great Circle sentence. It is
// the fuller of the two bearing-and-distance sentences: it includes the
// waypoint's own coordinates, not just its name.
//
//	$GPBWC,081837,4917.24,N,12309.57,W,051.9,T,031.6,M,1.3,N,DEST
//
// Field layout:
//
//	0 UTC time        hhmmss.ss
//	1 waypoint lat    ddmm.mm
//	2 N or S
//	3 waypoint lon    dddmm.mm
//	4 E or W
//	5 bearing true    degrees
//	6 reference       T
//	7 bearing magnetic degrees
//	8 reference       M
//	9 distance        nautical miles
//	10 units          N
//	11 waypoint id
//	12 FAA mode       NMEA 2.3 and later, optional
//
// Every field after the time is optional in practice, because a receiver that
// has lost the waypoint, or has not computed a bearing yet, still emits the
// sentence with the rest blank. Only the time is required.
type BWC struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms come from
	// the embedded LatLon. The position is the waypoint's, not the vessel's.
	LatLon
	// UTC is the time the bearing and distance were computed, which is not
	// the time the waypoint was defined.
	UTC nmea.TOD
	// BearingTrue and BearingMagnetic, in degrees, with presence flags. The
	// reference flags are validated, because reading a magnetic bearing as a
	// true one is a silent error of ten or more degrees.
	BearingTrue     float64
	HasBearingTrue  bool
	BearingMagnetic float64
	HasBearingMag   bool
	// Distance is nautical miles.
	Distance    float64
	HasDistance bool
	WaypointID  string
	Mode        string
	// Model records which sentence this is, so a caller handling BWC and
	// BWR together can tell how the distance was computed.
	Model string
}

type bwc struct{}

func (bwc) Formatter() string { return "BWC" }

func (bwc) Decode(s nmea.Sentence) (any, error) {
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out, err := decodeBearingAndDistance(s, "BWC")
	if err != nil {
		return out, err
	}
	out.Model = GreatCircle
	return out, nil
}

// decodeBearingAndDistance reads the shared body of BWC and BWR, whose
// layouts are byte for byte identical. Only the distance model differs,
// between a great circle and a rhumb line, so there is no reason to keep two
// copies of the field logic.
func decodeBearingAndDistance(s nmea.Sentence, formatter string) (BWC, error) {
	var out BWC
	var err error

	if out.UTC, err = s.Time(0); err != nil {
		return out, err
	}
	// The waypoint position is only present when the receiver has it, so both
	// halves are optional and are validated independently.
	if out.LatLon, err = latLonOptional(s, 1); err != nil {
		return out, err
	}
	if out.BearingTrue, out.HasBearingTrue, err = optionalFloat(s, 5); err != nil {
		return out, err
	}
	if out.HasBearingTrue && !referenceOK(s.Field(6), "T") {
		return out, errMislabeled(formatter, "true bearing", s.Field(6))
	}
	if out.BearingMagnetic, out.HasBearingMag, err = optionalFloat(s, 7); err != nil {
		return out, err
	}
	if out.HasBearingMag && !referenceOK(s.Field(8), "M") {
		return out, errMislabeled(formatter, "magnetic bearing", s.Field(8))
	}
	if out.Distance, out.HasDistance, err = optionalFloat(s, 9); err != nil {
		return out, err
	}
	if out.HasDistance && !referenceOK(s.Field(10), "N") {
		return out, errMislabeled(formatter, "distance", s.Field(10))
	}
	out.WaypointID = text(s.Field(11))
	out.Mode = text(s.Field(12))
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence.
func (b BWC) NMEASupports() []nmea.Feature {
	if b.Mode != "" {
		return nmea.FeatureOnly(nmea.FeatureFAA)
	}
	return nil
}
