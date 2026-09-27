package sentences

import (
	"testing"
)

// TestPublishedVHW verifies the water speed and heading layout against the
// documented field list:
//
//	$--VHW,x.x,T,x.x,M,x.x,N,x.x,K
//
// gpsd publishes no worked example for VHW, so the values below come from the
// field list and from a real receiver sentence:
//
//	$IIVHW,100.0,T,105.0,M,5.5,N,10.2,K*63
func TestPublishedVHW(t *testing.T) {
	v := published[VHW](t, "IIVHW,100.0,T,105.0,M,5.5,N,10.2,K")

	// Field 0: heading relative to true north.
	if !v.HasHeadingTrue || !near(v.HeadingTrue, 100.0, 1e-9) {
		t.Errorf("field 0 true heading = %v (present %v), want 100.0", v.HeadingTrue, v.HasHeadingTrue)
	}
	// Field 2: the same heading relative to magnetic north.
	if !v.HasHeadingMag || !near(v.HeadingMagnetic, 105.0, 1e-9) {
		t.Errorf("field 2 magnetic heading = %v (present %v), want 105.0", v.HeadingMagnetic, v.HasHeadingMag)
	}
	// Fields 4 and 6: the same speed twice, in knots and in km/h. Both are
	// kept as sent rather than one being derived from the other, because a
	// receiver may round either.
	if !v.HasSpeed {
		t.Fatal("HasSpeed = false for a sentence carrying a speed")
	}
	if !near(v.SpeedKnots, 5.5, 1e-9) {
		t.Errorf("field 4 speed = %v knots, want 5.5", v.SpeedKnots)
	}
	if !near(v.SpeedKmh, 10.2, 1e-9) {
		t.Errorf("field 6 speed = %v km/h, want 10.2", v.SpeedKmh)
	}
}

// TestVHWRejectsMislabeledReferences checks that the two heading references are
// not interchangeable. A magnetic heading differs from a true one by the local
// variation, so accepting 100.0 M as a true heading would put the vessel
// several degrees off, which is exactly the error a compass calibration
// depends on catching.
func TestVHWRejectsMislabeledReferences(t *testing.T) {
	if err := publishedErr(t, "IIVHW,100.0,M,105.0,M,5.5,N,10.2,K"); err == nil {
		t.Error("decoding a true heading labelled M succeeded, want an error")
	}
	if err := publishedErr(t, "IIVHW,100.0,T,105.0,T,5.5,N,10.2,K"); err == nil {
		t.Error("decoding a magnetic heading labelled T succeeded, want an error")
	}
}

// TestVHWSpeedOnly covers the partial sentence. A receiver with a log but no
// compass sends the speed and leaves the headings blank, which is a normal
// state rather than a truncated sentence.
func TestVHWSpeedOnly(t *testing.T) {
	// Fields 0 to 3 are the true heading and its reference, blank because
	// there is no compass, so the sentence starts with five commas: the one
	// that separates the address from field 0 plus one per blank field.
	v := published[VHW](t, "IIVHW,,,,,5.5,N,10.2,K")

	if v.HasHeadingTrue || v.HasHeadingMag {
		t.Error("a speed-only VHW reported a heading")
	}
	if !v.HasSpeed || !near(v.SpeedKnots, 5.5, 1e-9) {
		t.Errorf("speed = %v (present %v), want 5.5", v.SpeedKnots, v.HasSpeed)
	}
}
