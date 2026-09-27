package sentences

import (
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// WPL is the Waypoint Location sentence. It defines a waypoint: a name bound
// to a position. Sentences of this form are how routes are loaded into a
// receiver and read back out of it.

//------------------------------------------------------------------------------------------
//	$GPWPL,4917.16,N,12310.64,W,003*65
//------------------------------------------------------------------------------------------

// Field layout:
//
//	0 latitude       ddmm.mmmm
//	1 latitude hemi  N or S
//	2 longitude      dddmm.mmmm
//	3 longitude hemi E or W
//	4 waypoint name  up to 5 characters in NMEA 0183 v2, longer in practice
//
// A WPL carries a position but is a definition, not an observation. It
// therefore does not implement nmea.FixContributor, so loading a route full of
// waypoints cannot move the reported position.
type WPL struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms
	// come from the embedded LatLon.
	LatLon
	// Name identifies the waypoint, trimmed of padding.
	Name string
}

type wpl struct{}

func (wpl) Formatter() string { return "WPL" }

func (wpl) Decode(s nmea.Sentence) (any, error) {
	// The waypoint name is part of the definition, not decoration, so it is
	// required.
	if !s.HasFields(5) {
		return nil, needFields(s, 5)
	}
	out := WPL{Base: newBase(s)}

	out.LatLon = latLonFrom(s, 0)
	out.Name = text(s.Field(4))
	return out, nil
}

// Encode renders the waypoint back into a WPL sentence with a correct
// checksum, which is what a program needs in order to send a waypoint to a
// receiver. Pass an empty talker for the generic "GP".
//
// The raw ddmm.mmmm values are used rather than the decimal degrees, so a
// waypoint read out of one receiver and written to another is byte
// identical, leading zeros and all.
//
//	$GPWPL,4917.1600,N,12310.6400,W,003*4C
func (w WPL) Encode(talker string) string {
	if talker == "" {
		talker = "GP"
	}
	lat, lon := w.LatitudeRaw, w.LongitudeRaw
	if !lat.Valid {
		// Fall back to the decimal degrees, which are all that is available
		// for a waypoint a program built itself.
		lat = nmea.Coordinate{
			Degrees:    int(abs(w.Latitude)),
			Minutes:    abs(w.Latitude-float64(int(abs(w.Latitude)))) * 60,
			Hemisphere: hemisphereFromSign(w.Latitude, nmea.North, nmea.South),
			Valid:      true,
		}
		lon = nmea.Coordinate{
			Degrees:    int(abs(w.Longitude)),
			Minutes:    abs(w.Longitude-float64(int(abs(w.Longitude)))) * 60,
			Hemisphere: hemisphereFromSign(w.Longitude, nmea.East, nmea.West),
			Valid:      true,
		}
	}
	body := talker + "WPL," + strings.Join([]string{
		lat.ValueString(),
		string(rune(hemisphereOr(lat.Hemisphere, nmea.North))),
		lon.ValueString(),
		string(rune(hemisphereOr(lon.Hemisphere, nmea.East))),
		w.Name,
	}, ",")
	return nmea.Frame(body)
}

// hemisphereFromSign picks the hemisphere letter a signed degree value needs.
func hemisphereFromSign(degrees float64, positive, negative nmea.Hemisphere) nmea.Hemisphere {
	if degrees < 0 {
		return negative
	}
	return positive
}

// hemisphereOr returns h, or fallback when h is unset. Encoding a coordinate
// with no hemisphere would produce a sentence a receiver cannot use, so a
// default keeps the output well formed.
func hemisphereOr(h, fallback nmea.Hemisphere) nmea.Hemisphere {
	if h == nmea.HemisphereUnknown {
		return fallback
	}
	return h
}
