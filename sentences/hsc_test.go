package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// HSC decoder tests.

// TestPublishedHSC verifies the heading steering command layout. The two
// figures differ by the local magnetic variation, so both are kept.
func TestPublishedHSC(t *testing.T) {
	h := published[HSC](t, "IIHSC,123.4,T,124.5,M")

	if !h.HasHeadingTrue || h.HeadingTrue != 123.4 {
		t.Errorf("field 0 true heading = %v (present %v), want 123.4", h.HeadingTrue, h.HasHeadingTrue)
	}
	if !h.HasHeadingMag || h.HeadingMagnetic != 124.5 {
		t.Errorf("field 2 magnetic heading = %v (present %v), want 124.5",
			h.HeadingMagnetic, h.HasHeadingMag)
	}
	if h.DataType() != "HSC" {
		t.Errorf("DataType = %q, want HSC", h.DataType())
	}
}

// TestHSCOneHeadingOnly covers a half-populated sentence, which a receiver sends
// when it knows only one north.
func TestHSCOneHeadingOnly(t *testing.T) {
	trueOnly := published[HSC](t, "IIHSC,123.4,T")
	if !trueOnly.HasHeadingTrue || trueOnly.HasHeadingMag {
		t.Errorf("true-only decode = %+v, want the magnetic heading absent", trueOnly)
	}

	magOnly := published[HSC](t, "IIHSC,,T,124.5,M")
	if magOnly.HasHeadingTrue || !magOnly.HasHeadingMag {
		t.Errorf("magnetic-only decode = %+v, want the true heading absent", magOnly)
	}
}

// TestHSCTruncated covers a sentence with no heading at all.
func TestHSCTruncated(t *testing.T) {
	if err := publishedErr(t, "IIHSC"); err == nil {
		t.Error("decoding an HSC with no fields succeeded, want an error")
	}
}

// TestHSCIsNotAnObservation is the property that matters for an autopilot. HSC
// is a command, not a measurement, so folding it into the fix would have a
// vessel's autopilot reporting a heading it has not yet turned to.
func TestHSCIsNotAnObservation(t *testing.T) {
	var v any = published[HSC](t, "IIHSC,123.4,T,124.5,M")
	if _, ok := v.(nmea.FixContributor); ok {
		t.Error("HSC implements FixContributor, but it is a command rather than a measurement")
	}
}

// TestHSCString covers the log line, including the case where the receiver sent
// nothing usable.
func TestHSCString(t *testing.T) {
	if got := published[HSC](t, "IIHSC,123.4,T,124.5,M").String(); got != "HSC true 123.4 mag 124.5" {
		t.Errorf("String = %q", got)
	}
	if got := published[HSC](t, "IIHSC,123.4,T").String(); got != "HSC true 123.4" {
		t.Errorf("String = %q, want the true-only form", got)
	}
	if got := published[HSC](t, "IIHSC,,T,,M").String(); got != "HSC no heading" {
		t.Errorf("String = %q, want the empty form", got)
	}
}
