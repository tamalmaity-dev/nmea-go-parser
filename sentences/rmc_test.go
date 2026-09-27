package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestRMC(t *testing.T) {
	r := decodeOne[RMC](t, "$GPRMC,161229.487,A,3723.2475,N,12158.3416,W,0.13,309.62,120598,,*10")

	if r.Status != nmea.StatusValid {
		t.Errorf("Status = %v, want valid", r.Status)
	}
	lat, lon, ok := r.Latitude, r.Longitude, r.HasPosition
	if !ok {
		t.Fatal("Position reported no position")
	}
	if math.Abs(lat-37.3874583) > 1e-6 || math.Abs(lon+121.97236) > 1e-5 {
		t.Errorf("position = %v, %v, want 37.3874583, -121.97236", lat, lon)
	}
	if !r.HasSpeed || math.Abs(r.SpeedKnots-0.13) > 1e-9 {
		t.Errorf("SpeedKnots = %v (present %v), want 0.13", r.SpeedKnots, r.HasSpeed)
	}
	// The unit conversions must agree with each other exactly.
	if math.Abs(r.SpeedKmh-0.13*1.852) > 1e-9 {
		t.Errorf("SpeedKmh = %v, want %v", r.SpeedKmh, 0.13*1.852)
	}
	if math.Abs(r.SpeedMps-0.13*1852.0/3600.0) > 1e-9 {
		t.Errorf("SpeedMps = %v, want %v", r.SpeedMps, 0.13*1852.0/3600.0)
	}
	if !r.HasCourse || math.Abs(r.CourseDegrees-309.62) > 1e-9 {
		t.Errorf("CourseDegrees = %v (present %v), want 309.62", r.CourseDegrees, r.HasCourse)
	}
	if !r.Date.Valid || r.Date.Year != 1998 || r.Date.Month != 5 || r.Date.Day != 12 {
		t.Errorf("Date = %+v, want 1998-05-12", r.Date)
	}
	// The magnetic variation is blank here, so it must be absent, not zero.
	if r.HasMagVariation {
		t.Error("HasMagVariation = true for a blank field, want false")
	}

	ts, ok := r.Timestamp()
	if !ok {
		t.Fatal("Timestamp unavailable, want 1998-05-12 16:12:29")
	}
	if ts.Year() != 1998 || ts.Month() != 5 || ts.Day() != 12 ||
		ts.Hour() != 16 || ts.Minute() != 12 || ts.Second() != 29 {
		t.Errorf("Timestamp = %v, want 1998-05-12 16:12:29", ts)
	}
}
func TestRMCMagneticVariationSign(t *testing.T) {
	// West variation is negative. Getting this backwards is a classic bug
	// and it is invisible until a compass is involved.
	west := decodeOne[RMC](t, nmea.Frame("GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W"))
	if !west.HasMagVariation || math.Abs(west.MagneticVariation-(-3.1)) > 1e-9 {
		t.Errorf("west variation = %v (present %v), want -3.1", west.MagneticVariation, west.HasMagVariation)
	}

	east := decodeOne[RMC](t, nmea.Frame("GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,E"))
	if !east.HasMagVariation || math.Abs(east.MagneticVariation-3.1) > 1e-9 {
		t.Errorf("east variation = %v (present %v), want 3.1", east.MagneticVariation, east.HasMagVariation)
	}
}
func TestRMCVoidStatus(t *testing.T) {
	// Status V with a blank position is the normal "no fix" sentence. It
	// must decode, and it must invalidate the fix rather than leave a stale
	// position live.
	r := decodeOne[RMC](t, "$GPRMC,103607.00,V,,,,,,,,,,N*7E")
	if r.Status != nmea.StatusInvalid {
		t.Errorf("Status = %v, want invalid", r.Status)
	}

	var fix nmea.Fix
	fix.Valid = true
	fix.HasPosition = true
	fix.Latitude, fix.Longitude = 51.5, -0.1
	r.ApplyFix(&fix)

	if fix.Valid {
		t.Error("a void RMC left the fix valid, want invalid")
	}
	// The previous position must survive, so a caller can see where the
	// receiver last thought it was.
	if !fix.HasPosition {
		t.Error("a void RMC erased the last known position, want it retained")
	}
}
func TestRMCNavStatus(t *testing.T) {
	// Field 12 uses S/C/U/V. The A/D/E/M/N/S/V alphabet belongs to a
	// different field, and using it here would report a Caution as
	// Autonomous and an Unsafe as Manual.
	r := decodeOne[RMC](t, "$GNRMC,115522.000,A,4006.20885,N,11628.14498,E,0.000,0.50,041215,,,A,S*30")

	if !r.HasNavStatus || r.NavStatus != nmea.NavStatusSafe {
		t.Errorf("NavStatus = %v (present %v), want safe", r.NavStatus, r.HasNavStatus)
	}
	if r.Status != nmea.StatusValid {
		t.Errorf("Status = %v, want valid", r.Status)
	}
	if r.Mode != "A" {
		t.Errorf("Mode = %q, want A", r.Mode)
	}

	var fix nmea.Fix
	r.ApplyFix(&fix)
	if !fix.Valid {
		t.Error("a safe RMC did not produce a valid fix")
	}
}
func TestRMCUnsafeNavStatusOverridesStatusA(t *testing.T) {
	// Status A with navigational status U is the case that matters: the
	// receiver has a fix and is telling you not to use it.
	r := decodeOne[RMC](t, nmea.Frame("GNRMC,115522.000,A,4006.20885,N,11628.14498,E,0.000,0.50,041215,,,A,U"))
	if r.Status != nmea.StatusValid {
		t.Fatalf("Status = %v, want valid", r.Status)
	}
	if r.NavStatus != nmea.NavStatusUnsafe {
		t.Fatalf("NavStatus = %v, want unsafe", r.NavStatus)
	}

	var fix nmea.Fix
	r.ApplyFix(&fix)
	if fix.Valid {
		t.Error("status A with navigational status U produced a valid fix")
	}
}

func TestParseNavStatusRejectsFAAModeAlphabet(t *testing.T) {
	// A/D/E/M/N are FAA mode letters, not navigational statuses. Accepting
	// them here is the documented misimplementation, so they must be
	// rejected rather than silently mapped.
	for _, in := range []string{"A", "D", "E", "M", "N"} {
		if _, err := nmea.ParseNavStatus(in); err == nil {
			t.Errorf("ParseNavStatus(%q) succeeded, want an error: it is an FAA mode letter", in)
		}
	}
	for in, want := range map[string]nmea.NavStatus{
		"S": nmea.NavStatusSafe, "C": nmea.NavStatusCaution,
		"U": nmea.NavStatusUnsafe, "V": nmea.NavStatusNotValid,
	} {
		got, err := nmea.ParseNavStatus(in)
		if err != nil || got != want {
			t.Errorf("ParseNavStatus(%q) = %v, %v; want %v, nil", in, got, err, want)
		}
	}
	// Caution and Unknown must not report as navigable: treating "I do not
	// know" as permission to steer is the dangerous default.
	if nmea.NavStatusCaution.Navigable() {
		t.Error("a caution status reports navigable")
	}
	if nmea.NavStatusUnknown.Navigable() {
		t.Error("an unknown status reports navigable")
	}
	if !nmea.NavStatusSafe.Navigable() {
		t.Error("a safe status reports not navigable")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedRMC verifies the twelve-field layout including the NMEA 4.10
// navigational status at field 12.
func TestPublishedRMC(t *testing.T) {
	// Published example.
	r := published[RMC](t, "GNRMC,001031.00,A,4404.13993,N,12118.86023,W,0.146,,100117,,,A")

	// Field 0: UTC. Field 1: status A.
	if r.UTC.Hour != 0 || r.UTC.Minute != 10 || r.UTC.Second != 31 {
		t.Errorf("field 0 UTC = %v, want 00:10:31", r.UTC)
	}
	if r.Status != nmea.StatusValid {
		t.Errorf("field 1 status = %v, want valid", r.Status)
	}
	// Fields 2 to 5: 44 degrees 04.13993' N, 121 degrees 18.86023' W.
	if !near(r.Latitude, 44+4.13993/60, 1e-9) {
		t.Errorf("field 2 latitude = %v, want %v", r.Latitude, 44+4.13993/60)
	}
	if !near(r.Longitude, -(121 + 18.86023/60), 1e-9) {
		t.Errorf("field 4 longitude = %v, want %v", r.Longitude, -(121 + 18.86023/60))
	}
	// Field 6: speed over ground in knots. Field 7: track, blank here.
	if !near(r.SpeedKnots, 0.146, 1e-9) {
		t.Errorf("field 6 speed = %v, want 0.146", r.SpeedKnots)
	}
	if r.HasCourse {
		t.Errorf("field 7 course = %v, want absent", r.CourseDegrees)
	}
	// Field 8: the date, ddmmyy.
	if !r.Date.Valid || r.Date.Year != 2017 || r.Date.Month != 1 || r.Date.Day != 10 {
		t.Errorf("field 8 date = %+v, want 2017-01-10", r.Date)
	}
	// Fields 9 and 10: magnetic variation and its hemisphere, blank here.
	if r.HasMagVariation {
		t.Errorf("field 9 variation = %v, want absent", r.MagneticVariation)
	}
	// Field 11: the FAA mode. Field 12: the navigational status, absent
	// here because this example predates NMEA 4.10.
	if r.Mode != "A" {
		t.Errorf("field 11 FAA mode = %q, want A", r.Mode)
	}
	if r.HasNavStatus {
		t.Errorf("field 12 nav status = %v, want absent", r.NavStatus)
	}
}
