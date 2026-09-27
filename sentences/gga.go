package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// GGA is the Global Positioning System Fix Data sentence. It is the most
// commonly watched sentence in existence, because it is the one that carries
// altitude and the fix-quality indicator alongside the position.
//
//	$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18
//
// Field layout:
//
//	 0 UTC time          hhmmss.ss
//	 1 latitude          ddmm.mmmm
//	 2 latitude hemi     N or S
//	 3 longitude         dddmm.mmmm
//	 4 longitude hemi    E or W
//	 5 fix quality       0-8, see nmea.FixQuality
//	 6 satellites used   count
//	 7 HDOP              horizontal dilution of precision
//	 8 altitude          metres above mean sea level
//	 9 altitude unit     normally M
//	10 geoid separation  metres, MSL to ellipsoid
//	11 geoid unit        normally M
//	12 DGPS age          seconds since last correction
//	13 DGPS station id   0000-1023
//
// Every optional field has a Has flag. A receiver searching for satellites
// sends GGA with the position blanked, and an altitude of exactly 0.0 is a
// real altitude, so presence has to be explicit rather than inferred from a
// zero value.
type GGA struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms come from
	// the embedded LatLon.
	LatLon
	// UTC is the time of day the fix was taken.
	UTC nmea.TOD
	// Quality describes how good the fix is: autonomous, differential,
	// RTK, and so on.
	Quality nmea.FixQuality
	// SatellitesUsed is how many satellites contributed to the fix.
	SatellitesUsed    int
	HasSatellitesUsed bool
	// HDOP is horizontal dilution of precision; below 2 is a single-point
	// fix, below 1 is good.
	HDOP    float64
	HasHDOP bool
	// Altitude is metres above mean sea level.
	Altitude    float64
	HasAltitude bool
	// GeoidSeparation is metres from mean sea level to the ellipsoid, so
	// altitude above the ellipsoid is Altitude + GeoidSeparation.
	GeoidSeparation    float64
	HasGeoidSeparation bool
	// DGPSAge is seconds since the last differential correction, valid
	// only when DGPSStationID is set.
	DGPSAge    float64
	HasDGPSAge bool
	// DGPSStationID identifies the correction source, 0-1023.
	DGPSStationID    int
	HasDGPSStationID bool
}

type gga struct{}

func (gga) Formatter() string { return nmea.TypeGGA }

func (gga) Decode(s nmea.Sentence) (any, error) {
	// A GGA with no position at all is not a decode failure: a receiver
	// still searching for satellites sends exactly that, with the numeric
	// fields blank. Only the first field is genuinely required.
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := GGA{Base: newBase(s)}

	var err error
	if out.UTC, err = s.Time(0); err != nil {
		return out, err
	}
	out.LatLon = latLonFrom(s, 1)

	if out.Quality, err = s.Quality(5); err != nil {
		return out, err
	}
	if out.SatellitesUsed, out.HasSatellitesUsed, err = optionalInt(s, 6); err != nil {
		return out, err
	}
	if out.HDOP, out.HasHDOP, err = optionalFloat(s, 7); err != nil {
		return out, err
	}
	if out.Altitude, out.HasAltitude, err = optionalFloat(s, 8); err != nil {
		return out, err
	}
	if out.GeoidSeparation, out.HasGeoidSeparation, err = optionalFloat(s, 10); err != nil {
		return out, err
	}
	if out.DGPSAge, out.HasDGPSAge, err = optionalFloat(s, 12); err != nil {
		return out, err
	}
	if out.DGPSStationID, out.HasDGPSStationID, err = optionalInt(s, 13); err != nil {
		return out, err
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: a GGA with a field past 13
// only occurs from NMEA 4.10, though what it means is receiver specific.
func (g GGA) NMEASupports() []nmea.Feature {
	if len(g.Sentence.Fields) > 14 {
		return nmea.FeatureOnly(nmea.FeatureSystemID)
	}
	return nil
}

// ApplyFix folds the GGA into the fix state.
//
// GGA is the only sentence that reports fix quality, so it also decides
// whether the fix counts as valid. A receiver that loses the fix sends
// quality 0 with the position still populated, and this is where that
// correctly invalidates the fix instead of leaving a stale position live.
func (g GGA) ApplyFix(f *nmea.Fix) {
	f.Quality = g.Quality
	f.Valid = g.Quality.Fixed() && g.HasPosition

	if applyPosition(f, g.LatLon) && g.HasSatellitesUsed {
		f.SatellitesUsed, f.HasSatellitesUsed = g.SatellitesUsed, true
	}
	if g.UTC.Available {
		f.TimeOfDay = g.UTC
	}
	if g.HasHDOP {
		f.HDOP, f.HasHDOP = g.HDOP, true
	}
	if g.HasAltitude {
		f.Altitude, f.HasAltitude = g.Altitude, true
	}
	if g.HasGeoidSeparation {
		f.GeoidHeight, f.HasGeoidHeight = g.GeoidSeparation, true
	}
}
