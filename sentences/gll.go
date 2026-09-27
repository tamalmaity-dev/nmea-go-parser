package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// GLL is the Geographic Position, Latitude/Longitude sentence. It is the
// smallest of the position sentences: position, time, and status, with no
// altitude, no dilution of precision, and no velocity.

//------------------------------------------------------------------------------------------
//	$GPGLL,4916.45,N,12311.12,W,225444,A,A*5E
//------------------------------------------------------------------------------------------

// Field layout:
//
//	0 latitude       ddmm.mmmm
//	1 latitude hemi  N or S
//	2 longitude      dddmm.mmmm
//	3 longitude hemi E or W
//	4 UTC time       hhmmss.ss
//	5 status         A valid, V void
//	6 FAA mode       receiver specific
//
// Receivers configured for a bare position fix often emit only GLL, so it
// matters even though it is the poorest of the three.
type GLL struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms
	// come from the embedded LatLon.
	LatLon
	// UTC is the time of day the fix was taken.
	UTC nmea.TOD
	// Status is the receiver's verdict on the fix.
	Status nmea.NavigationStatus
	// Mode is the FAA mode indicator, receiver specific.
	Mode string
}

// NMEASupports implements nmea.FeatureEvidence: the trailing mode field
// arrived in NMEA 2.3.
func (g GLL) NMEASupports() []nmea.Feature {
	if g.Mode != "" {
		return nmea.FeatureOnly(nmea.FeatureFAA)
	}
	return nil
}

type gll struct{}

func (gll) Formatter() string { return "GLL" }

func (gll) Decode(s nmea.Sentence) (any, error) {
	// Without fields 0 to 3 there is no position at all.
	if !s.HasFields(4) {
		return nil, needFields(s, 4)
	}
	out := GLL{Base: newBase(s)}

	out.LatLon = latLonFrom(s, 0)

	var err error
	if out.UTC, err = s.Time(4); err != nil {
		return out, err
	}
	if out.Status, err = s.Status(5); err != nil {
		return out, err
	}
	out.Mode = text(s.Field(6))
	return out, nil
}

// ApplyFix folds the GLL into the fix state. GLL reports no fix quality of
// its own, so it updates the position and validity without touching quality;
// a GGA in the same epoch is what establishes quality.
func (g GLL) ApplyFix(f *nmea.Fix) {
	if g.Status != nmea.StatusValid {
		f.Valid = false
		return
	}
	if !applyPosition(f, g.LatLon) {
		return
	}
	f.Valid = true
	if g.UTC.Available {
		f.TimeOfDay = g.UTC
	}
}
