package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestRMB(t *testing.T) {
	// The layout that matters here: the coordinates in fields 5 to 8 are
	// the DESTINATION waypoint, and fields 3 and 4 are the origin and
	// destination names.
	r := decodeOne[RMB](t, "$GPRMB,A,0.66,L,003,004,4917.24,N,12309.57,W,001.3,052.5,000.5,V*20")

	if r.Status != nmea.StatusValid {
		t.Errorf("Status = %v, want valid", r.Status)
	}
	if !r.HasCrossTrack || math.Abs(r.CrossTrack-0.66) > 1e-9 {
		t.Errorf("CrossTrack = %v, want 0.66", r.CrossTrack)
	}
	if r.Steer != nmea.SideLeft {
		t.Errorf("Steer = %v, want left", r.Steer)
	}
	// Left of track is negative, which is the convention charting tools
	// expect, so the sign must be applied here rather than left to the caller.
	if math.Abs(r.CrossTrackSigned-(-0.66)) > 1e-9 {
		t.Errorf("CrossTrackSigned = %v, want -0.66", r.CrossTrackSigned)
	}
	if r.OriginID != "003" || r.DestinationID != "004" {
		t.Errorf("origin/destination = %q/%q, want 003/004", r.OriginID, r.DestinationID)
	}

	// The coordinates are the destination waypoint, not the vessel.
	lat, lon, ok := r.Destination.Latitude, r.Destination.Longitude, r.Destination.HasPosition
	if !ok {
		t.Fatal("the destination position was not decoded")
	}
	if math.Abs(lat-49.28733) > 1e-4 || math.Abs(lon+123.1595) > 1e-4 {
		t.Errorf("destination = %v, %v, want 49.28733, -123.1595", lat, lon)
	}

	if !r.HasRangeToDestination || math.Abs(r.RangeToDestination-1.3) > 1e-9 {
		t.Errorf("RangeToDestination = %v, want 1.3", r.RangeToDestination)
	}
	if !r.HasBearingToDestination || math.Abs(r.BearingToDestination-52.5) > 1e-9 {
		t.Errorf("BearingToDestination = %v, want 52.5", r.BearingToDestination)
	}
	if !r.HasClosingVelocity || math.Abs(r.ClosingVelocity-0.5) > 1e-9 {
		t.Errorf("ClosingVelocity = %v, want 0.5", r.ClosingVelocity)
	}
	if r.Arrived() {
		t.Error("Arrived = true for a V arrival flag, want false")
	}
}
func TestRMBDoesNotTouchTheFix(t *testing.T) {
	// If RMB implemented FixContributor, the parser would fold its fields
	// into the fix and the reported position would jump onto the waypoint.
	if _, ok := interface{}(&RMB{}).(nmea.FixContributor); ok {
		t.Error("RMB implements FixContributor, so it can overwrite the reported position")
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedRMB is the layout most often mis-implemented. Fields 5 to 8
// are the DESTINATION waypoint, not the vessel, which is why RMB does not
// implement nmea.FixContributor at all.
func TestPublishedRMB(t *testing.T) {
	// Published example.
	r := published[RMB](t, "GPRMB,A,0.66,L,003,004,4917.24,N,12309.57,W,001.3,052.5,000.5,V")

	// Field 0: status A, active.
	if r.Status != nmea.StatusValid {
		t.Errorf("field 0 status = %v, want valid", r.Status)
	}
	// Field 1: cross-track error in nautical miles. Field 2: steer left.
	if !near(r.CrossTrack, 0.66, 1e-9) {
		t.Errorf("field 1 cross-track = %v, want 0.66", r.CrossTrack)
	}
	if r.Steer != nmea.SideLeft {
		t.Errorf("field 2 steer = %v, want left", r.Steer)
	}
	if !near(r.CrossTrackSigned, -0.66, 1e-9) {
		t.Errorf("cross-track signed = %v, want -0.66 for a port error", r.CrossTrackSigned)
	}
	// Fields 3 and 4: the origin and destination waypoint names. The origin
	// comes first, which is the opposite of the BOD order.
	if r.OriginID != "003" {
		t.Errorf("field 3 origin = %q, want 003", r.OriginID)
	}
	if r.DestinationID != "004" {
		t.Errorf("field 4 destination = %q, want 004", r.DestinationID)
	}
	// Fields 5 to 8: the destination's position, 49 deg 17.24' N.
	if !near(r.Destination.Latitude, 49+17.24/60, 1e-9) {
		t.Errorf("field 5 destination latitude = %v, want %v", r.Destination.Latitude, 49+17.24/60)
	}
	if !near(r.Destination.Longitude, -(123 + 9.57/60), 1e-9) {
		t.Errorf("field 7 destination longitude = %v, want %v",
			r.Destination.Longitude, -(123 + 9.57/60))
	}
	if r.Destination.HasPosition != true {
		t.Error("field 5/7 destination position was not populated")
	}
	// Field 9: range in nautical miles. Field 10: bearing, degrees true.
	if !near(r.RangeToDestination, 1.3, 1e-9) {
		t.Errorf("field 9 range = %v, want 1.3", r.RangeToDestination)
	}
	if !near(r.BearingToDestination, 52.5, 1e-9) {
		t.Errorf("field 10 bearing = %v, want 52.5", r.BearingToDestination)
	}
	// Field 11: closing velocity, knots. Field 12: arrival status V.
	if !near(r.ClosingVelocity, 0.5, 1e-9) {
		t.Errorf("field 11 closing velocity = %v, want 0.5", r.ClosingVelocity)
	}
	if r.Arrived() {
		t.Error("field 12 arrival status V reported as arrived")
	}
	// Field 13, the FAA mode, is absent from this example.
	if r.Mode != "" {
		t.Errorf("field 13 mode = %q, want empty", r.Mode)
	}

	// The whole point: this sentence must never set the fix position, or
	// the reported position jumps onto the waypoint.
	if _, isContributor := any(r).(nmea.FixContributor); isContributor {
		t.Error("RMB implements FixContributor, so it can overwrite the reported position")
	}
}
