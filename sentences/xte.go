package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// XTE is the Cross-Track Error, Measured sentence. It is the smallest of the
// cross-track sentences: a distance, a direction, and a unit.
//
//	$GPXTE,A,A,0.67,L,N
//	$GPXTE,V,V,,,N,S
//
// Field layout:
//
//	0 status         A valid, V warning or no reliable fix
//	1 status         A ok, V Loran-C cycle lock warning
//	2 cross-track error magnitude
//	3 direction to steer L or R
//	4 units          N nautical miles, K kilometres
//	5 FAA mode       NMEA 2.3 and later, optional
//
// This sentence carries no position at all, which is what distinguishes it
// from RMB and APB. A program that needs a fix must get it from GGA, RMC, or
// GNS.
type XTE struct {
	Base
	// Status is the overall validity of the guidance.
	Status nmea.NavigationStatus
	// LockStatus reports Loran-C cycle lock, which does not apply to GNSS
	// and is normally A.
	LockStatus nmea.NavigationStatus
	// CrossTrack is the distance off track, and CrossTrackSigned applies the
	// steer direction so that a negative value always means the vessel is to
	// port of the course line.
	CrossTrack       float64
	CrossTrackSigned float64
	HasCrossTrack    bool
	// Steer is the direction to steer to return to the course line.
	Steer nmea.Side
	// Unit is N for nautical miles or K for kilometres.
	Unit DistanceUnit
	// Mode is the FAA mode indicator, NMEA 2.3 and later.
	Mode string
}

// CrossTrackNauticalMiles returns the signed cross-track error in nautical
// miles, converted from kilometres when that is the unit sent.
func (x *XTE) CrossTrackNauticalMiles() (nm float64, ok bool) {
	if !x.HasCrossTrack {
		return 0, false
	}
	return x.Unit.ToNauticalMiles(x.CrossTrackSigned), true
}

// NMEASupports implements nmea.FeatureEvidence: the FAA mode arrived in
// NMEA 2.3.
func (x XTE) NMEASupports() []nmea.Feature {
	if x.Mode != "" {
		return nmea.FeatureOnly(nmea.FeatureFAA)
	}
	return nil
}

type xte struct{}

func (xte) Formatter() string { return "XTE" }

func (xte) Decode(s nmea.Sentence) (any, error) {
	if !s.HasFields(4) {
		return nil, needFields(s, 4)
	}
	out := XTE{Base: newBase(s)}

	var err error
	if out.Status, err = s.Status(0); err != nil {
		return out, err
	}
	if out.LockStatus, err = s.Status(1); err != nil {
		return out, err
	}
	if out.CrossTrack, out.HasCrossTrack, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.HasCrossTrack {
		if out.Steer, err = nmea.ParseSide(s.Field(3)); err != nil {
			return out, err
		}
		if out.Steer == nmea.SideLeft {
			out.CrossTrackSigned = -out.CrossTrack
		} else {
			out.CrossTrackSigned = out.CrossTrack
		}
	}
	if out.Unit, err = ParseDistanceUnit(s.Field(4)); err != nil {
		return out, err
	}
	out.Mode = text(s.Field(5))
	return out, nil
}
