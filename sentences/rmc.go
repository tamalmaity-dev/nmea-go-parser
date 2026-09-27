package sentences

import (
	"fmt"
	"time"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// RMC is the Recommended Minimum Navigation Information sentence. Every
// conformant receiver emits it, and it is the only sentence that carries the
// full date alongside the time, which makes it the sentence to use when a
// fix has to be timestamped.

//------------------------------------------------------------------------------------------
//	$GPRMC,123519,A,4807.038,N,01131.000,E,022.4,084.4,230394,003.1,W*6A
//------------------------------------------------------------------------------------------

// Field layout:
//
//	 0 UTC time          hhmmss.ss
//	 1 status            A valid, V void
//	 2 latitude          ddmm.mmmm
//	 3 latitude hemi     N or S
//	 4 longitude         dddmm.mmmm
//	 5 longitude hemi    E or W
//	 6 speed over ground knots
//	 7 course over ground degrees true
//	 8 date              ddmmyy
//	 9 magnetic variation degrees
//	10 magnetic var hemi E or W
//	11 FAA mode indicator, receiver specific, NMEA 2.3 and later
//	12 nav status        S safe, C caution, U unsafe, V not valid, NMEA 4.10
//
// Field 12 is the navigational status, and its alphabet is S/C/U/V. It is
// routinely mis-implemented using the FAA mode alphabet of A/D/E/M/N/S/V,
// which is a different field with different meanings: doing so would report
// a Caution as Autonomous and an Unsafe as Manual, both of which sound
// reassuring. See nmea.NavStatus.
type RMC struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms come from
	// the embedded LatLon. The position is kept even when Status is invalid,
	// because the values are often still useful for diagnosing a receiver.
	LatLon
	// UTC is the time of day the fix was taken.
	UTC nmea.TOD
	// Status is the receiver's own verdict on the fix. A void status with
	// populated coordinates is how a receiver reports a stale position, so
	// check it rather than trusting the coordinates alone.
	Status nmea.NavigationStatus
	// SpeedKnots is ground speed over water, with km/h and m/s
	// precomputed.
	SpeedKnots float64
	SpeedKmh   float64
	SpeedMps   float64
	HasSpeed   bool
	// CourseDegrees is course over ground, degrees true, 0 to 360.
	CourseDegrees float64
	HasCourse     bool
	// Date is the UTC date. Combined with UTC it gives a full timestamp,
	// which GGA alone cannot provide.
	Date nmea.Date
	// MagneticVariation is degrees, signed positive for east following the
	// hemisphere field. West variation is negative.
	MagneticVariation float64
	HasMagVariation   bool
	// Mode is the FAA mode indicator. Its meaning is receiver specific and
	// only reliably available on receivers that document it.
	Mode string
	// NavStatus is the NMEA 4.10 navigational status, which says whether
	// the fix is safe to navigate on, not merely whether one exists.
	NavStatus    nmea.NavStatus
	HasNavStatus bool
}

// NMEASupports implements nmea.FeatureEvidence. The FAA mode field arrived in
// NMEA 2.3 and the navigational status in 4.10, so the presence of each dates
// the receiver.
func (r RMC) NMEASupports() []nmea.Feature {
	switch {
	case r.HasNavStatus:
		return nmea.FeaturesNavAndFAA()
	case r.Mode != "":
		return nmea.FeatureOnly(nmea.FeatureFAA)
	default:
		return nil
	}
}

type rmc struct{}

func (rmc) Formatter() string { return "RMC" }

func (rmc) Decode(s nmea.Sentence) (any, error) {
	// Fields 0 through 10 are the whole sentence; a shorter one is truncated
	// and cannot be trusted.
	if !s.HasFields(9) {
		return nil, needFields(s, 9)
	}
	out := RMC{Base: newBase(s)}

	t, err := s.Time(0)
	if err != nil {
		return out, err
	}
	out.UTC = t

	if out.Status, err = s.Status(1); err != nil {
		return out, err
	}
	out.LatLon = latLonFrom(s, 2)

	if out.SpeedKnots, out.HasSpeed, err = optionalFloat(s, 6); err != nil {
		return out, err
	}
	if out.HasSpeed {
		out.SpeedKmh = out.SpeedKnots * 1.852
		out.SpeedMps = out.SpeedKnots * 1852.0 / 3600.0
	}

	if out.CourseDegrees, out.HasCourse, err = optionalFloat(s, 7); err != nil {
		return out, err
	}
	// A course of exactly 360 is out of range; 0 is a legal north heading.
	if out.HasCourse && (out.CourseDegrees < 0 || out.CourseDegrees > 360) {
		return out, fmt.Errorf("sentences: RMC course %v out of range 0..360: %w",
			out.CourseDegrees, nmea.ErrFieldRange)
	}

	if out.Date, err = s.Date(8); err != nil {
		return out, err
	}

	// Magnetic variation arrives as a magnitude plus an E/W flag, and a
	// western variation is negative.
	if v, present, verr := optionalFloat(s, 9); verr != nil {
		return out, verr
	} else if present {
		if nmea.ParseHemisphere(s.Field(10)) == nmea.West {
			v = -v
		}
		out.MagneticVariation, out.HasMagVariation = v, true
	}

	out.Mode = text(s.Field(11))

	// Field 12 is the NMEA 4.10 navigational status, optional on every
	// receiver that predates it.
	if !s.Blank(12) {
		nav, err := nmea.ParseNavStatus(s.Field(12))
		if err != nil {
			return out, err
		}
		out.NavStatus, out.HasNavStatus = nav, true
	}
	return out, nil
}

// ApplyFix folds the RMC into the fix state.
//
// RMC carries the date, so it is the sentence that makes a full timestamp
// possible. Its status is authoritative: a void status invalidates the fix
// even though GGA may have reported quality a moment earlier.
//
// The NMEA 4.10 navigational status is stricter still. A receiver may
// happily report status A while marking the fix unsafe, which is exactly the
// case a program must not navigate on, so an unsafe or not-valid status
// invalidates the fix regardless of the A/V field.
func (r RMC) ApplyFix(f *nmea.Fix) {
	f.Valid = r.Status == nmea.StatusValid
	if r.HasNavStatus && !r.NavStatus.Navigable() {
		f.Valid = false
	}

	if applyPosition(f, r.LatLon) {
		if r.HasSpeed {
			f.SetSpeedKnots(r.SpeedKnots)
		}
		if r.HasCourse {
			f.CourseDegrees, f.HasCourse = r.CourseDegrees, true
		}
	}
	if r.UTC.Available {
		f.TimeOfDay = r.UTC
	}
	if r.Date.Available {
		f.Date = r.Date
	}
	if r.HasMagVariation {
		f.MagneticVariation, f.HasMagVariation = r.MagneticVariation, true
	}
	if r.HasNavStatus {
		f.NavStatus, f.HasNavStatus = r.NavStatus, true
	}
}

// Timestamp returns the full date and time of the fix, and false when the
// receiver has not sent enough information to build one.
func (r *RMC) Timestamp() (t time.Time, ok bool) { return r.Date.Combine(r.UTC) }
