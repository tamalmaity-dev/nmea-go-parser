package sentences

import (
	"math"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// GST is the GNSS Pseudorange Noise and Error Statistics sentence. Unlike
// GGA, which reports dilution of precision, GST reports the actual expected
// error of the fix in metres, broken down by axis.
//
//	$GPGST,182141.000,15.5,15.3,7.2,21.8,0.9,0.5,0.8*54
//	$GPGST,082356.00,1.8,,,,1.7,1.3,2.2*7E
//
// Field layout:
//
//	0 UTC time           of the associated GGA or GNS fix
//	1 RMS std deviation  of the range inputs, metres
//	2 std dev semi-major axis of the error ellipse, metres
//	3 std dev semi-minor axis, metres
//	4 orientation of the semi-major axis, degrees true
//	5 std dev latitude error,  metres
//	6 std dev longitude error, metres
//	7 std dev altitude error, metres
//
// Blank fields are common and normal: a receiver with a 2D fix has no
// altitude error, and a 3D fix computed from few satellites may have no
// error ellipse. Every value therefore has a presence flag, and the ellipse
// fields are only meaningful together.
type GST struct {
	Base
	// UTC is the time of the fix these statistics describe, not the time
	// they were computed. That lets a caller match them to a GGA.
	UTC nmea.TOD
	// RMS is the total range residual standard deviation in metres, which
	// reflects measurement noise rather than geometry.
	RMS    float64
	HasRMS bool
	// SemiMajor and SemiMinor are the standard deviations of the error
	// ellipse axes in metres, and Orientation is the bearing of the
	// semi-major axis in degrees true. They describe a horizontal error
	// ellipse, which is what a consumer needs for a target area.
	SemiMajor      float64
	HasSemiMajor   bool
	SemiMinor      float64
	HasSemiMinor   bool
	Orientation    float64
	HasOrientation bool
	// ErrorLatitude, ErrorLongitude, and ErrorAltitude are the per-axis
	// standard deviations in metres.
	ErrorLatitude  float64
	HasErrorLat    bool
	ErrorLongitude float64
	HasErrorLon    bool
	ErrorAltitude  float64
	HasErrorAlt    bool
}

// PositionError returns the horizontal position error in metres as the
// root-sum-square of the latitude and longitude standard deviations, which is
// how a receiver's per-axis figures combine into a single radius.
//
// It reports false unless both axes are present, because half an error
// estimate is not an error estimate.
func (g *GST) PositionError() (metres float64, ok bool) {
	if !g.HasErrorLat || !g.HasErrorLon {
		return 0, false
	}
	return math.Hypot(g.ErrorLatitude, g.ErrorLongitude), true
}

// VerticalError returns the altitude standard deviation in metres.
func (g *GST) VerticalError() (metres float64, ok bool) {
	return g.ErrorAltitude, g.HasErrorAlt
}

// Ellipse returns the horizontal error ellipse in metres and degrees true,
// and false unless both axes are present.
func (g *GST) Ellipse() (major, minor, orientation float64, ok bool) {
	if !g.HasSemiMajor || !g.HasSemiMinor {
		return 0, 0, 0, false
	}
	return g.SemiMajor, g.SemiMinor, g.Orientation, true
}

type gst struct{}

func (gst) Formatter() string { return "GST" }

func (gst) Decode(s nmea.Sentence) (any, error) {
	// The time is required; everything after it is a statistic a receiver may
	// legitimately leave blank.
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := GST{Base: newBase(s)}

	var err error
	if out.UTC, err = s.Time(0); err != nil {
		return out, err
	}

	// Each figure is read independently, because a partial GST is normal and
	// must not fail the whole sentence.
	for _, f := range []struct {
		index  int
		value  *float64
		has    *bool
		absent error
	}{
		{1, &out.RMS, &out.HasRMS, nil},
		{2, &out.SemiMajor, &out.HasSemiMajor, nil},
		{3, &out.SemiMinor, &out.HasSemiMinor, nil},
		{4, &out.Orientation, &out.HasOrientation, nmea.ErrFieldValue},
		{5, &out.ErrorLatitude, &out.HasErrorLat, nil},
		{6, &out.ErrorLongitude, &out.HasErrorLon, nil},
		{7, &out.ErrorAltitude, &out.HasErrorAlt, nil},
	} {
		v, present, err := optionalFloat(s, f.index)
		if err != nil {
			if f.absent != nil {
				continue
			}
			return out, err
		}
		if !present {
			continue
		}
		// A bearing outside a full turn is meaningless, and GST orientation
		// is the one field that is a true bearing rather than a magnitude.
		if f.index == 4 && (v < 0 || v > 360) {
			return out, errRange("GST", "error ellipse orientation", v, "0..360")
		}
		*f.value, *f.has = v, true
	}
	return out, nil
}

// ApplyFix folds the GST into the fix state.
//
// GST carries no position of its own, only the expected error of one, so it
// contributes accuracy rather than a location. A receiver that emits GST is
// also at least NMEA 3.0, since the sentence did not exist before then.
func (g GST) ApplyFix(f *nmea.Fix) {
	if g.UTC.Available {
		f.TimeOfDay = g.UTC
	}
	if e, ok := g.PositionError(); ok {
		f.HorizontalError, f.HasHorizontalError = e, true
	}
	if e, ok := g.VerticalError(); ok {
		f.VerticalError, f.HasVerticalError = e, true
	}
}
