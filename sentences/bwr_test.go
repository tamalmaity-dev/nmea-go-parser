package sentences

import (
	"testing"
)

// TestPublishedBWR verifies the rhumb line bearing and distance layout, which
// is the same field list as BWC with a different geometry:
//
//	$--BWR,hhmmss.ss,llll.ll,a,yyyyy.yy,a,x.x,T,x.x,M,x.x,N,c--c
//
// gpsd publishes no worked example, so the values come from the field list and
// from a real receiver sentence:
//
//	$GPBWR,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,4.6,N,POINT*4F
func TestPublishedBWR(t *testing.T) {
	b := published[BWC](t, "GPBWR,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,4.6,N,POINT")

	// BWR and BWC decode to the same value; the geometry is the only
	// difference, so this asserts it explicitly.
	if b.Model != RhumbLine {
		t.Errorf("Model = %v, want RhumbLine", b.Model)
	}
	if b.UTC.Hour != 22 || b.UTC.Minute != 5 || b.UTC.Second != 16 {
		t.Errorf("field 0 UTC = %v, want 22:05:16", b.UTC)
	}
	// Fields 1 to 4: the waypoint position. The western longitude is
	// negative, because the hemisphere is part of the value.
	if !near(b.Latitude, 51+30.02/60, 1e-9) {
		t.Errorf("field 1 latitude = %v, want %v", b.Latitude, 51+30.02/60)
	}
	if !near(b.Longitude, -(0 + 46.34/60), 1e-9) {
		t.Errorf("field 3 longitude = %v, want %v", b.Longitude, -(0 + 46.34/60))
	}
	// Fields 5 to 9: the two bearings and the distance.
	if !near(b.BearingTrue, 213.8, 1e-9) {
		t.Errorf("field 5 true bearing = %v, want 213.8", b.BearingTrue)
	}
	if !near(b.BearingMagnetic, 218.0, 1e-9) {
		t.Errorf("field 7 magnetic bearing = %v, want 218.0", b.BearingMagnetic)
	}
	if !b.HasDistance || !near(b.Distance, 4.6, 1e-9) {
		t.Errorf("field 9 distance = %v (present %v), want 4.6", b.Distance, b.HasDistance)
	}
	// Field 11: the waypoint name.
	if b.WaypointID != "POINT" {
		t.Errorf("field 11 waypoint = %q, want POINT", b.WaypointID)
	}
}

// TestBWRIsNotGreatCircle is the one thing that separates BWR from BWC. A
// program that treats the two as interchangeable gets noticeably wrong
// answers on long legs, because a rhumb line and a great circle diverge by
// several degrees at sea.
func TestBWRIsNotGreatCircle(t *testing.T) {
	rhumb := published[BWC](t, "GPBWR,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,4.6,N,POINT")
	great := published[BWC](t, "GPBWC,220516,5130.02,N,00046.34,W,213.8,T,218.0,M,4.6,N,POINT")

	if rhumb.Model != RhumbLine {
		t.Errorf("BWR Model = %v, want RhumbLine", rhumb.Model)
	}
	if great.Model != GreatCircle {
		t.Errorf("BWC Model = %v, want GreatCircle", great.Model)
	}
	// The shared fields must decode identically, so the geometry flag is the
	// only difference between the two sentences.
	if rhumb.WaypointID != great.WaypointID || rhumb.BearingTrue != great.BearingTrue {
		t.Error("the shared fields of BWR and BWC decoded differently")
	}
}
