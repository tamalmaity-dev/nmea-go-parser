package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// APB is the Autopilot Sentence B, the fixed-form guidance sentence that
// autopilots consume. It gives a cross-track error with a steer direction and
// then three separate bearings, each with its own true-or-magnetic reference.

//------------------------------------------------------------------------------------------
//	$GPAPB,A,A,0.10,R,N,V,V,011,M,DEST,011,M,011,M*57
//------------------------------------------------------------------------------------------

// Field layout, per the standard:
//
//	 0 status overall   A data valid, V warning
//	 1 status bearing   A ok, V cycle-lock warning
//	 2 cross-track err  magnitude
//	 3 steer direction  L port, R starboard
//	 4 cross-track unit N nautical miles, K kilometres
//	 5 status arrival   A arrival circle entered
//	 6 status perpen.   A perpendicular passed
//	 7 bearing          origin to destination
//	 8 reference        T true, M magnetic
//	 9 destination id   destination waypoint name
//	10 bearing          present position to destination
//	11 reference        T or M
//	12 heading          heading to steer to reach the destination
//	13 reference        T or M
//
// The three bearings are the field that trips people up. Field 7 is the leg
// as planned, field 10 is the bearing from where the vessel actually is now,
// and field 12 is what the helm should be turned to. They coincide only when
// the vessel is exactly on the course line, which is why each is kept
// separate here rather than collapsed into one "bearing" field.
type APB struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms
	// come from the embedded LatLon.
	LatLon // Status is the overall validity flag. A void status means the autopilot
	// should ignore the guidance entirely.
	Status nmea.NavigationStatus
	// BearingStatus says whether the bearing fields may be trusted.
	BearingStatus nmea.NavigationStatus
	// CrossTrack is the distance off track, and CrossTrackSigned applies the
	// steer sense to it, so a negative value always means port.
	CrossTrack       float64
	CrossTrackSigned float64
	HasCrossTrack    bool
	// Steer is the direction the vessel is off track toward.
	Steer nmea.Side
	// CrossTrackUnit is N for nautical miles or K for kilometres.
	CrossTrackUnit DistanceUnit
	// Arrival and Perpendicular are the two arrival status flags.
	Arrival       nmea.StatusFlag
	Perpendicular nmea.StatusFlag

	// BearingOriginToDestination is the planned course for the leg, and
	// BearingOriginTrue reports whether it is referenced to true north.
	BearingOriginToDestination float64
	HasBearingOrigin           bool
	BearingOriginTrue          bool

	// BearingToDestination is the bearing from the vessel's present
	// position, which differs from the planned leg as soon as the vessel
	// drifts off the line.
	BearingToDestination float64
	HasBearingToDest     bool
	BearingToDestTrue    bool

	// HeadingToSteer is what the helm should be set to, and
	// HeadingToSteerTrue reports its reference.
	HeadingToSteer     float64
	HasHeadingToSteer  bool
	HeadingToSteerTrue bool

	// DestinationID is the destination waypoint name.
	DestinationID string

	// Some receivers append the destination coordinates after the standard
	// fourteen fields. They are decoded when present, which is the only way
	// to get a waypoint position out of APB at all, since the standard form
	// does not carry one.
	Destination LatLon
}

// StandardAPBFields is the number of payload fields in the standard form,
// before any receiver-specific extension.
const StandardAPBFields = 14

type apb struct{}

func (apb) Formatter() string { return "APB" }

func (apb) Decode(s nmea.Sentence) (any, error) {
	// The destination name closes the standard form, so anything shorter
	// cannot be steered by.
	if !s.HasFields(StandardAPBFields - 1) {
		return nil, needFields(s, StandardAPBFields-1)
	}
	out := APB{Base: newBase(s)}

	var err error
	if out.Status, err = s.Status(0); err != nil {
		return out, err
	}
	if out.BearingStatus, err = s.Status(1); err != nil {
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
	if out.CrossTrackUnit, err = ParseDistanceUnit(s.Field(4)); err != nil {
		return out, err
	}

	out.Arrival = nmea.ParseStatusFlag(s.Field(5))
	out.Perpendicular = nmea.ParseStatusFlag(s.Field(6))

	// Each bearing is a value followed by a T/M reference. The reference is
	// validated rather than ignored: reading a magnetic heading as a true one
	// is a silent error that produces a perfectly plausible number.
	type bearing struct {
		value      float64
		has        bool
		trueNorth  bool
		valueField int
		refField   int
		what       string
	}
	bearings := []bearing{
		{valueField: 7, refField: 8, what: "origin to destination"},
		{valueField: 10, refField: 11, what: "position to destination"},
		{valueField: 12, refField: 13, what: "heading to steer"},
	}
	for _, b := range bearings {
		v, present, err := optionalFloat(s, b.valueField)
		if err != nil {
			return out, err
		}
		if !present {
			continue
		}
		ref := s.Field(b.refField)
		switch ref {
		case "T", "t":
			b.trueNorth = true
		case "M", "m":
			b.trueNorth = false
		default:
			return out, errMislabeled("APB", b.what+" bearing", ref)
		}
		b.value, b.has = v, true

		switch b.valueField {
		case 7:
			out.BearingOriginToDestination, out.HasBearingOrigin, out.BearingOriginTrue = b.value, b.has, b.trueNorth
		case 10:
			out.BearingToDestination, out.HasBearingToDest, out.BearingToDestTrue = b.value, b.has, b.trueNorth
		case 12:
			out.HeadingToSteer, out.HasHeadingToSteer, out.HeadingToSteerTrue = b.value, b.has, b.trueNorth
		}
	}

	out.DestinationID = text(s.Field(9))

	// Extension: destination coordinates, past the standard fourteen fields.
	if s.HasFields(StandardAPBFields+4) && !s.Blank(StandardAPBFields) {
		if out.Destination, err = latLonOptional(s, StandardAPBFields); err != nil {
			return out, err
		}
	}
	return out, nil
}

// CrossTrackNauticalMiles returns the cross-track error in nautical miles,
// converting from kilometres when that is the unit the receiver used.
func (a *APB) CrossTrackNauticalMiles() (nm float64, ok bool) {
	if !a.HasCrossTrack {
		return 0, false
	}
	return a.CrossTrackUnit.ToNauticalMiles(a.CrossTrackSigned), true
}
