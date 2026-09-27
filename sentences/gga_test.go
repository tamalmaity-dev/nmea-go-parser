package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGGA(t *testing.T) {
	// Published example: 37 deg 23.2475' N, 121 deg 58.3416' W, 9.0 m MSL.
	g := decodeOne[GGA](t, "$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18")

	if !g.UTC.Available || !g.UTC.Valid {
		t.Fatalf("UTC = %+v, want 16:12:29.487", g.UTC)
	}
	if g.UTC.Hour != 16 || g.UTC.Minute != 12 || g.UTC.Second != 29 {
		t.Errorf("UTC = %v, want 16:12:29", g.UTC)
	}

	lat, lon, ok := g.Latitude, g.Longitude, g.HasPosition
	if !ok {
		t.Fatal("Position reported no position")
	}
	// 37 + 23.2475/60 and -(121 + 58.3416/60)
	if math.Abs(lat-37.3874583) > 1e-6 {
		t.Errorf("lat = %v, want 37.3874583", lat)
	}
	if math.Abs(lon+121.97236) > 1e-5 {
		t.Errorf("lon = %v, want -121.97236", lon)
	}

	if g.Quality != nmea.QualityGPS {
		t.Errorf("Quality = %v, want gps", g.Quality)
	}
	if g.SatellitesUsed != 7 || !g.HasSatellitesUsed {
		t.Errorf("SatellitesUsed = %d (present %v), want 7", g.SatellitesUsed, g.HasSatellitesUsed)
	}
	if !g.HasHDOP || math.Abs(g.HDOP-1.0) > 1e-9 {
		t.Errorf("HDOP = %v (present %v), want 1.0", g.HDOP, g.HasHDOP)
	}
	if !g.HasAltitude || math.Abs(g.Altitude-9.0) > 1e-9 {
		t.Errorf("Altitude = %v (present %v), want 9.0", g.Altitude, g.HasAltitude)
	}
	// The geoid separation is blank in this example, so it must be reported
	// as absent rather than as zero.
	if g.HasGeoidSeparation {
		t.Errorf("HasGeoidSeparation = true for a blank field, want false")
	}
	// The station id is the documented null value 0000: present, but zero.
	if g.DGPSStationID != 0 {
		t.Errorf("DGPSStationID = %d, want 0", g.DGPSStationID)
	}
}
func TestGGAWithoutFix(t *testing.T) {
	// A receiver searching for satellites sends quality 0 with the position
	// blanked. It must decode without error and must not claim a position.
	g := decodeOne[GGA](t, "$GPGGA,103607.00,,,,,0,00,99.9,,,,,,,*70")

	if g.Quality != nmea.QualityNone {
		t.Errorf("Quality = %v, want none", g.Quality)
	}
	if g.HasPosition {
		t.Error("blank position decoded as a valid coordinate")
	}

	var fix nmea.Fix
	g.ApplyFix(&fix)
	if fix.Valid {
		t.Error("fix is valid from a quality-0 GGA, want invalid")
	}
	if fix.HasPosition {
		t.Error("fix gained a position from a blank GGA")
	}
}
func TestGGAZeroAltitudeIsNotAbsent(t *testing.T) {
	// An altitude of exactly 0.0 m is a real altitude, at sea level, and
	// must survive the round trip rather than being read as "not reported".
	g := decodeOne[GGA](t, nmea.Frame("GPGGA,103607.00,5327.03942,N,00214.42462,W,1,05,4.6,0.0,M,50.5,M,,"))
	if !g.HasAltitude {
		t.Error("HasAltitude = false for an altitude of 0.0, want true")
	}
	if g.Altitude != 0 {
		t.Errorf("Altitude = %v, want 0", g.Altitude)
	}
}
func TestWatchTypedHandler(t *testing.T) {
	// Watch is the ergonomic path a caller uses to get a typed value without
	// a type assertion, so it needs to actually deliver the right type.
	p := newTestParser()

	var got []GGA
	Watch(p, "GGA", func(g GGA) error {
		got = append(got, g)
		return nil
	})

	var want int
	for _, line := range sampleLines(t) {
		s, err := nmea.ParseSentence(line)
		if err != nil {
			continue
		}
		if s.Type == "GGA" && s.ChecksumOK() {
			want++
		}
		if _, err := p.ParseLine(line); err != nil {
			continue
		}
	}
	if want == 0 {
		t.Fatal("the sample data has no GGA sentences")
	}
	if len(got) != want {
		t.Fatalf("GGA handler called %d times, want %d", len(got), want)
	}
	if !got[0].HasPosition {
		t.Error("handler received an invalid latitude")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGGA verifies the field list:
// 0 UTC, 1 lat, 2 N/S, 3 lon, 4 E/W, 5 quality, 6 satellites,
// 7 HDOP, 8 altitude, 9 unit, 10 geoid, 11 unit, 12 DGPS age, 13 station.
func TestPublishedGGA(t *testing.T) {
	// Published example.
	g := published[GGA](t, "GNGGA,001043.00,4404.14036,N,12118.85961,W,1,12,0.98,1113.0,M,-21.3,M")

	if !near(g.UTC.Duration().Seconds(), 10*60+43, 0.01) {
		t.Errorf("field 0 UTC = %v, want 00:10:43", g.UTC)
	}
	// Fields 1 and 2: 44 degrees 04.14036 minutes north.
	if !near(g.Latitude, 44+4.14036/60, 1e-9) {
		t.Errorf("field 1 latitude = %v, want %v", g.Latitude, 44+4.14036/60)
	}
	if g.LatitudeRaw.Hemisphere != nmea.North {
		t.Errorf("field 2 hemisphere = %v, want N", g.LatitudeRaw.Hemisphere)
	}
	// Fields 3 and 4: 121 degrees 18.85961 minutes west, so negative.
	if !near(g.Longitude, -(121 + 18.85961/60), 1e-9) {
		t.Errorf("field 3 longitude = %v, want %v", g.Longitude, -(121 + 18.85961/60))
	}
	if g.LongitudeRaw.Hemisphere != nmea.West {
		t.Errorf("field 4 hemisphere = %v, want W", g.LongitudeRaw.Hemisphere)
	}
	// Field 5: quality 1, an autonomous GPS fix.
	if g.Quality != nmea.QualityGPS {
		t.Errorf("field 5 quality = %v, want gps", g.Quality)
	}
	// Field 6: twelve satellites, the top of the range GGA allows.
	if g.SatellitesUsed != 12 {
		t.Errorf("field 6 satellites = %d, want 12", g.SatellitesUsed)
	}
	// Field 7: HDOP.
	if !near(g.HDOP, 0.98, 1e-9) {
		t.Errorf("field 7 HDOP = %v, want 0.98", g.HDOP)
	}
	// Field 8: altitude above mean sea level. Field 9 is the unit and is
	// deliberately not surfaced.
	if !near(g.Altitude, 1113.0, 1e-9) {
		t.Errorf("field 8 altitude = %v, want 1113.0", g.Altitude)
	}
	// Field 10: geoidal separation, negative here because mean sea level is
	// below the ellipsoid. Field 11 is its unit.
	if !near(g.GeoidSeparation, -21.3, 1e-9) {
		t.Errorf("field 10 geoid separation = %v, want -21.3", g.GeoidSeparation)
	}
	// Fields 12 and 13 are blank, so they must be absent rather than zero.
	if g.HasDGPSAge || g.HasDGPSStationID {
		t.Error("fields 12 and 13 are blank but were reported as present")
	}
}
