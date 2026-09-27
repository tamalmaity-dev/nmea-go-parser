package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// RMB is the Recommended Minimum Navigation Information sentence for a
// destination waypoint. It reports how far the vessel is off the course line
// to the destination and how far along it still has to go.
//
//	$GPRMB,A,0.66,L,003,004,4917.24,N,12309.57,W,001.3,052.5,000.5,V*0B
//
// Field layout, per the standard:
//
//	 0 status            A active, V invalid
//	 1 cross-track error nautical miles
//	 2 steer direction   L port, R starboard
//	 3 origin id         the waypoint just left
//	 4 destination id    the waypoint being steered to
//	 5 dest latitude     ddmm.mmmm   position OF THE WAYPOINT
//	 6 N or S
//	 7 dest longitude    dddmm.mmmm
//	 8 E or W
//	 9 range to dest     nautical miles
//	10 bearing to dest   degrees true
//	11 closing velocity  knots, positive when closing
//	12 arrival status    A arrival circle entered
//	13 FAA mode          NMEA 2.3 and later, optional
//
// The subtlety that matters: the coordinates in fields 5 to 8 are the
// destination waypoint, not the vessel. Several widely-copied decoder
// examples read them as the vessel position, which silently moves the
// reported fix onto the waypoint. This type therefore does not implement
// nmea.FixContributor at all, so there is no way for an RMB to overwrite the
// position that GGA and RMC established. Vessel position comes from GGA,
// RMC, or GLL.
type RMB struct {
	Base
	// Destination is the waypoint's own position, in both decimal degrees
	// and the exact ddmm.mmmm form it arrived in. It is named rather than
	// embedded precisely because it is not the vessel's position: an
	// embedded LatLon here would invite exactly the mistake this decoder
	// exists to avoid.
	Destination LatLon
	// Status is the receiver's verdict on the guidance.
	Status nmea.NavigationStatus
	// CrossTrack is the distance off track in nautical miles, and
	// CrossTrackSigned applies the steer sense to it, so a negative value
	// always means the vessel is to port of the course line.
	CrossTrack       float64
	CrossTrackSigned float64
	HasCrossTrack    bool
	// Steer is the direction the vessel is off track toward.
	Steer nmea.Side
	// OriginID and DestinationID name the leg being navigated.
	OriginID      string
	DestinationID string
	// RangeToDestination is in nautical miles.
	RangeToDestination    float64
	HasRangeToDestination bool
	// BearingToDestination is degrees true.
	BearingToDestination    float64
	HasBearingToDestination bool
	// ClosingVelocity is knots, positive when closing on the waypoint and
	// negative when opening.
	ClosingVelocity    float64
	HasClosingVelocity bool
	// Arrival is A once the vessel is inside the arrival circle.
	Arrival nmea.StatusFlag
	// Mode is the FAA mode indicator, NMEA 2.3 and later.
	Mode string
}

type rmb struct{}

func (rmb) Formatter() string { return "RMB" }

func (rmb) Decode(s nmea.Sentence) (any, error) {
	// Fields 0 to 4 are the status, cross-track, and waypoint names, which
	// are what the sentence exists to report.
	if !s.HasFields(5) {
		return nil, needFields(s, 5)
	}
	out := RMB{Base: newBase(s)}

	var err error
	if out.Status, err = s.Status(0); err != nil {
		return out, err
	}

	if out.CrossTrack, out.HasCrossTrack, err = optionalFloat(s, 1); err != nil {
		return out, err
	}
	if out.HasCrossTrack {
		if out.Steer, err = nmea.ParseSide(s.Field(2)); err != nil {
			return out, err
		}
		// Left of track is negative, the sign convention every charting
		// tool expects. An absent steer flag leaves the magnitude unsigned
		// rather than inventing a direction.
		switch out.Steer {
		case nmea.SideLeft:
			out.CrossTrackSigned = -out.CrossTrack
		case nmea.SideRight:
			out.CrossTrackSigned = out.CrossTrack
		default:
			out.CrossTrackSigned = out.CrossTrack
		}
	}

	out.OriginID = text(s.Field(3))
	out.DestinationID = text(s.Field(4))

	// The destination position is optional. A receiver that has the waypoint
	// stored navigates to it without echoing the coordinates back, and blank
	// here is normal rather than malformed.
	if !s.Blank(5) {
		if out.Destination, err = latLonOptional(s, 5); err != nil {
			return out, err
		}
	}

	if out.RangeToDestination, out.HasRangeToDestination, err = optionalFloat(s, 9); err != nil {
		return out, err
	}
	if out.BearingToDestination, out.HasBearingToDestination, err = optionalFloat(s, 10); err != nil {
		return out, err
	}
	if out.ClosingVelocity, out.HasClosingVelocity, err = optionalFloat(s, 11); err != nil {
		return out, err
	}
	out.Arrival = nmea.ParseStatusFlag(s.Field(12))
	out.Mode = text(s.Field(13))
	return out, nil
}

// Arrived reports whether the vessel has entered the arrival circle.
func (r RMB) Arrived() bool { return r.Arrival.Valid() }
