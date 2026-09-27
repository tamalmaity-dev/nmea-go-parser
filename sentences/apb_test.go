package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestAPB(t *testing.T) {
	// The three bearing pairs are the field that trips people up: the
	// planned leg, the bearing from the present position, and the heading to
	// steer. They are only equal when the vessel is exactly on the line.
	a := decodeOne[APB](t, "$GPAPB,A,A,0.10,R,N,V,V,011,M,DEST,011,M,011,M*3C")

	if a.Status != nmea.StatusValid {
		t.Errorf("Status = %v, want valid", a.Status)
	}
	if a.BearingStatus != nmea.StatusValid {
		t.Errorf("BearingStatus = %v, want valid", a.BearingStatus)
	}
	if !a.HasCrossTrack || math.Abs(a.CrossTrack-0.10) > 1e-9 {
		t.Errorf("CrossTrack = %v, want 0.10", a.CrossTrack)
	}
	if a.Steer != nmea.SideRight {
		t.Errorf("Steer = %v, want right", a.Steer)
	}
	if a.CrossTrackUnit != UnitNauticalMiles {
		t.Errorf("CrossTrackUnit = %v, want nautical miles", a.CrossTrackUnit)
	}
	if a.Arrival != nmea.FlagNo {
		t.Errorf("Arrival = %q, want V", a.Arrival)
	}
	if a.Perpendicular != nmea.FlagNo {
		t.Errorf("Perpendicular = %q, want V", a.Perpendicular)
	}
	if !a.HasBearingOrigin || math.Abs(a.BearingOriginToDestination-11) > 1e-9 {
		t.Errorf("BearingOriginToDestination = %v, want 11", a.BearingOriginToDestination)
	}
	// M means magnetic. A magnetic bearing must never be reported as true.
	if a.BearingOriginTrue {
		t.Error("BearingOriginTrue = true for an M reference, want false")
	}
	if !a.HasBearingToDest || a.BearingToDestTrue {
		t.Errorf("BearingToDestination = %v (true %v), want 11 magnetic", a.BearingToDestination, a.BearingToDestTrue)
	}
	if !a.HasHeadingToSteer || a.HeadingToSteerTrue {
		t.Errorf("HeadingToSteer = %v (true %v), want 11 magnetic", a.HeadingToSteer, a.HeadingToSteerTrue)
	}
	if a.DestinationID != "DEST" {
		t.Errorf("DestinationID = %q, want DEST", a.DestinationID)
	}
}
func TestAPBTrueReference(t *testing.T) {
	a := decodeOne[APB](t, nmea.Frame("GPAPB,A,A,0.10,L,K,V,V,011,T,DEST,012,T,013,T"))
	if a.Steer != nmea.SideLeft {
		t.Errorf("Steer = %v, want left", a.Steer)
	}
	if !a.BearingOriginTrue || !a.BearingToDestTrue || !a.HeadingToSteerTrue {
		t.Error("T references were not reported as true")
	}
	// Kilometres must be converted rather than passed through as miles, and
	// the sign of the steer direction must survive the conversion: L means
	// port, so the signed cross-track is negative.
	nm, ok := a.CrossTrackNauticalMiles()
	if !ok || math.Abs(nm-(-0.1/1.852)) > 1e-9 {
		t.Errorf("CrossTrackNauticalMiles = %v (ok %v), want %v", nm, ok, -0.1/1.852)
	}
}
func TestAPBMislabelledBearingIsRejected(t *testing.T) {
	line := nmea.Frame("GPAPB,A,A,0.10,R,N,V,V,011,X,DEST,011,M,011,M")
	s, err := nmea.ParseSentence(line)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nmea.DefaultRegistry.Decode(s); err == nil {
		t.Error("a bearing with an X reference decoded without error, want an error")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedAPB verifies the fourteen-field layout with its three
// bearing/reference pairs, which is the other sentence most often
// mis-implemented.
func TestPublishedAPB(t *testing.T) {
	// Published example.
	a := published[APB](t, "GPAPB,A,A,0.10,R,N,V,V,011,M,DEST,011,M,011,M")

	// Fields 0 and 1: two status flags, both A here.
	if a.Status != nmea.StatusValid {
		t.Errorf("field 0 status = %v, want valid", a.Status)
	}
	if a.BearingStatus != nmea.StatusValid {
		t.Errorf("field 1 bearing status = %v, want valid", a.BearingStatus)
	}
	// Field 2: cross-track magnitude. Field 3: steer right. Field 4: unit N.
	if !near(a.CrossTrack, 0.10, 1e-9) {
		t.Errorf("field 2 cross-track = %v, want 0.10", a.CrossTrack)
	}
	if a.Steer != nmea.SideRight {
		t.Errorf("field 3 steer = %v, want right", a.Steer)
	}
	if a.CrossTrackUnit != UnitNauticalMiles {
		t.Errorf("field 4 unit = %v, want nautical miles", a.CrossTrackUnit)
	}
	// Fields 5 and 6: arrival circle and perpendicular, both V here.
	if a.Arrival != nmea.FlagNo {
		t.Errorf("field 5 arrival = %q, want V", a.Arrival)
	}
	if a.Perpendicular != nmea.FlagNo {
		t.Errorf("field 6 perpendicular = %q, want V", a.Perpendicular)
	}
	// Fields 7 and 8: bearing origin to destination, then its M/T reference.
	if !near(a.BearingOriginToDestination, 11, 1e-9) {
		t.Errorf("field 7 bearing = %v, want 11", a.BearingOriginToDestination)
	}
	if a.BearingOriginTrue {
		t.Error("field 8 reference M was reported as true")
	}
	// Field 9: the destination waypoint name. Not a bearing, and not a
	// coordinate: reading it as either is a visible corruption.
	if a.DestinationID != "DEST" {
		t.Errorf("field 9 destination = %q, want DEST", a.DestinationID)
	}
	// Fields 10 and 11: bearing from the present position.
	if !near(a.BearingToDestination, 11, 1e-9) {
		t.Errorf("field 10 bearing = %v, want 11", a.BearingToDestination)
	}
	if a.BearingToDestTrue {
		t.Error("field 11 reference M was reported as true")
	}
	// Fields 12 and 13: heading to steer.
	if !near(a.HeadingToSteer, 11, 1e-9) {
		t.Errorf("field 12 heading to steer = %v, want 11", a.HeadingToSteer)
	}
	if a.HeadingToSteerTrue {
		t.Error("field 13 reference M was reported as true")
	}
	// The standard form carries no waypoint position, so a program needing
	// one has to look elsewhere.
	if a.Destination.HasPosition {
		t.Error("a destination position was decoded from a standard 14-field APB")
	}
}
