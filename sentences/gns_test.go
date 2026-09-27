package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestGNSModePerConstellation(t *testing.T) {
	// The mode indicator is a positional string, one character per
	// constellation: GPS, GLONASS, Galileo, BeiDou, QZSS, NavIC. Reading it
	// as a single flag loses the information that makes GNS better than GGA.
	g := decodeOne[GNS](t, "$GPGNS,112257.00,3844.24011,N,00908.43828,W,AN,03,10.5,,,,,S*28")

	if g.ModeIndicator != "AN" {
		t.Fatalf("ModeIndicator = %q, want AN", g.ModeIndicator)
	}
	modes := g.ModePerConstellation()
	if len(modes) != 2 {
		t.Fatalf("got %d per-constellation modes, want 2: %+v", len(modes), modes)
	}
	if modes[0].Constellation != nmea.ConstellationGPS || modes[0].Mode != ModeAutonomous {
		t.Errorf("first character = %v/%v, want GPS/autonomous", modes[0].Constellation, modes[0].Mode)
	}
	if modes[1].Constellation != nmea.ConstellationGLONASS || modes[1].Mode != ModeNone {
		t.Errorf("second character = %v/%v, want GLONASS/no fix", modes[1].Constellation, modes[1].Mode)
	}

	// The navigational status is at field 12, after four empty fields.
	if !g.HasNavStatus || g.NavStatus != nmea.NavStatusSafe {
		t.Errorf("NavStatus = %v (present %v), want safe", g.NavStatus, g.HasNavStatus)
	}
	if !g.NavStatus.Navigable() {
		t.Error("a safe navigational status is not navigable")
	}
}
func TestGNSAllConstellations(t *testing.T) {
	// "ANNN" is the common four-constellation case: GPS autonomous, the
	// other three not yet contributing.
	g := decodeOne[GNS](t, "$GNGNS,103607.00,5327.03942,N,00214.42462,W,ANNN,07,1.2,71.0,50.5,,,S*05")
	modes := g.ModePerConstellation()
	if len(modes) != 4 {
		t.Fatalf("got %d modes for ANNN, want 4", len(modes))
	}
	want := []nmea.Constellation{
		nmea.ConstellationGPS, nmea.ConstellationGLONASS,
		nmea.ConstellationGalileo, nmea.ConstellationBeiDou,
	}
	for i, w := range want {
		if modes[i].Constellation != w {
			t.Errorf("mode %d = %v, want %v", i, modes[i].Constellation, w)
		}
	}
	// Only the first contributes a fix.
	if !modes[0].Mode.Fixed() {
		t.Error("the autonomous mode does not count as a fix")
	}
	for i := 1; i < 4; i++ {
		if modes[i].Mode.Fixed() {
			t.Errorf("mode %d = %v, want no fix", i, modes[i].Mode)
		}
	}
}
func TestGNSPositionAndFix(t *testing.T) {
	// The standard's own example, which has no navigational status: that
	// field only exists from NMEA 4.10.
	g := decodeOne[GNS](t, "$GPGNS,112257.00,3844.24011,N,00908.43828,W,AN,03,10.5,,*57")
	lat, lon, ok := g.Latitude, g.Longitude, g.HasPosition
	if !ok {
		t.Fatal("Position reported no position")
	}
	if math.Abs(lat-38.7373) > 1e-3 {
		t.Errorf("lat = %v, want 38.7373", lat)
	}
	if math.Abs(lon+9.1406) > 1e-3 {
		t.Errorf("lon = %v, want -9.1406", lon)
	}
	if g.SatellitesUsed != 3 || !g.HasSatellitesUsed {
		t.Errorf("SatellitesUsed = %d, want 3", g.SatellitesUsed)
	}
	if !g.HasHDOP || math.Abs(g.HDOP-10.5) > 1e-9 {
		t.Errorf("HDOP = %v, want 10.5", g.HDOP)
	}

	var fix nmea.Fix
	g.ApplyFix(&fix)
	if !fix.HasPosition || !fix.Valid {
		t.Errorf("fix = %+v, want a valid position", fix)
	}
	if math.Abs(fix.Latitude-lat) > 1e-9 {
		t.Errorf("fix latitude = %v, want %v", fix.Latitude, lat)
	}
}
func TestGNSUnsafeStatusInvalidatesFix(t *testing.T) {
	// An unsafe navigational status must override the A/V status. A receiver
	// can report a perfectly good fix and still say do not navigate on it.
	g := decodeOne[GNS](t, nmea.Frame("GNGNS,103607.00,5327.03942,N,00214.42462,W,ANNN,07,1.2,71.0,50.5,,,U"))
	if g.NavStatus != nmea.NavStatusUnsafe {
		t.Fatalf("NavStatus = %v, want unsafe", g.NavStatus)
	}
	var fix nmea.Fix
	fix.Valid = true
	g.ApplyFix(&fix)
	if fix.Valid {
		t.Error("a GNS marked unsafe left the fix valid")
	}
	if !fix.HasPosition {
		t.Error("the position was discarded; only the validity should change")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedGNS verifies the per-constellation mode indicator, which is
// a positional string rather than a single flag.
func TestPublishedGNS(t *testing.T) {
	// Published example.
	g := published[GNS](t, "GPGNS,112257.00,3844.24011,N,00908.43828,W,AN,03,10.5,,")

	// Field 0: UTC.
	if g.UTC.Hour != 11 || g.UTC.Minute != 22 || g.UTC.Second != 57 {
		t.Errorf("field 0 UTC = %v, want 11:22:57", g.UTC)
	}
	// Fields 1 to 4: 38 degrees 44.24011' N, 9 degrees 08.43828' W.
	if !near(g.Latitude, 38+44.24011/60, 1e-9) {
		t.Errorf("field 1 latitude = %v, want %v", g.Latitude, 38+44.24011/60)
	}
	if !near(g.Longitude, -(9 + 8.43828/60), 1e-9) {
		t.Errorf("field 3 longitude = %v, want %v", g.Longitude, -(9 + 8.43828/60))
	}
	// Field 5: the mode string, one character per constellation in a fixed
	// order. "AN" is GPS autonomous and GLONASS not in use.
	if g.ModeIndicator != "AN" {
		t.Fatalf("field 5 mode = %q, want AN", g.ModeIndicator)
	}
	modes := g.ModePerConstellation()
	if len(modes) != 2 {
		t.Fatalf("got %d per-constellation modes, want 2", len(modes))
	}
	if modes[0].Constellation != nmea.ConstellationGPS || modes[0].Mode != ModeAutonomous {
		t.Errorf("first character = %v/%v, want GPS/autonomous", modes[0].Constellation, modes[0].Mode)
	}
	if modes[1].Constellation != nmea.ConstellationGLONASS || modes[1].Mode != ModeNone {
		t.Errorf("second character = %v/%v, want GLONASS/no fix", modes[1].Constellation, modes[1].Mode)
	}
	// Field 6: satellites in use, three here, which GGA could not express.
	if g.SatellitesUsed != 3 {
		t.Errorf("field 6 satellites = %d, want 3", g.SatellitesUsed)
	}
	// Field 7: HDOP. Field 8: altitude. Field 9: geoid separation.
	if !near(g.HDOP, 10.5, 1e-9) {
		t.Errorf("field 7 HDOP = %v, want 10.5", g.HDOP)
	}
	if g.HasAltitude {
		t.Errorf("field 8 altitude = %v, want absent", g.Altitude)
	}
	if g.HasGeoidSeparation {
		t.Errorf("field 9 separation = %v, want absent", g.GeoidSeparation)
	}
}
