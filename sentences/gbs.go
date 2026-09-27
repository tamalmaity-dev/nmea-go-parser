package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// GBS is the GNSS Satellite Fault Detection sentence, better known by the
// acronym RAIM: Receiver Autonomous Integrity Monitoring. It is the only
// sentence that tells a receiver which satellite it believes is lying.
//
//	$GPGBS,125027,23.43,M,13.91,M,34.01,M*07
//	$GPGBS,125900.0,23.4,M,13.9,M,34.0,M,1,0.5,0.7*36
//
// Field layout:
//
//		0 UTC time             of the GGA or GNS fix this refers to
//		1 expected error lat   1-sigma, metres
//		2 units                M
//		3 expected error lon   1-sigma, metres
//		4 units                M
//		5 expected error alt   1-sigma, metres
//		6 units                M
//		7 failed satellite id  1-138, optional
//		8 probability of missed detection, optional
//		9 estimated bias on that satellite, metres, optional
//	 10 bias estimate standard deviation, optional
//
// Each error figure is followed by its own unit letter, so the values sit at
// the odd indices rather than running consecutively. Reading them as
// consecutive fields shifts everything after the first and puts a unit letter
// where a number belongs, which is the failure this decoder had before.
type GBS struct {
	Base
	// UTC is the time of the fix these statistics apply to.
	UTC nmea.TOD
	// ExpectedErrorLat, ExpectedErrorLon, and ExpectedErrorAlt are the
	// receiver's predicted 1-sigma errors in metres for the position it just
	// reported.
	ExpectedErrorLat float64
	HasExpectedLat   bool
	ExpectedErrorLon float64
	HasExpectedLon   bool
	ExpectedErrorAlt float64
	HasExpectedAlt   bool
	// FailedSatelliteID is the satellite the receiver believes is
	// malfunctioning, 1 to 138. It is absent when no fault was found.
	FailedSatelliteID    int
	HasFailedSatelliteID bool
	// ProbabilityOfMissedDetection is the chance the receiver got it wrong.
	// It is only meaningful alongside a failed satellite.
	ProbabilityOfMissedDetection    float64
	HasProbabilityOfMissedDetection bool
	// EstimatedBias and BiasStdDev describe the receiver's estimate of the
	// failed satellite's error, which a correction can then remove.
	EstimatedBias    float64
	HasEstimatedBias bool
	BiasStdDev       float64
	HasBiasStdDev    bool
	// SystemID and SignalID arrived in NMEA 4.10, on some receivers.
	SystemID    nmea.SystemID
	HasSystemID bool
	SignalID    nmea.SignalDigit
	HasSignalID bool
}

// Integrity reports whether the receiver established integrity, which is
// stronger than merely having a fix. A fix with no GBS at all has unknown
// integrity, so this returns false in that case too.
func (g *GBS) Integrity() bool {
	// A named failure means integrity was established and a fault found.
	// That is a working integrity check, and the fault is what the caller
	// needs to act on, so it is not the same as "not navigable".
	if g.HasFailedSatelliteID {
		return true
	}
	// No named fault but a stated missed-detection probability still means
	// the check ran. Zero probability with no failure is a clean pass.
	return g.HasProbabilityOfMissedDetection
}

// Faulty reports whether a specific satellite was blamed.
func (g *GBS) Faulty() bool { return g.HasFailedSatelliteID }

type gbs struct{}

func (gbs) Formatter() string { return "GBS" }

func (gbs) Decode(s nmea.Sentence) (any, error) {
	// The time and the three expected-error figures are the core; each error
	// is a value followed by its unit letter.
	if !s.HasFields(6) {
		return nil, needFields(s, 6)
	}
	out := GBS{Base: newBase(s)}

	var err error
	if out.UTC, err = s.Time(0); err != nil {
		return out, err
	}
	if out.ExpectedErrorLat, out.HasExpectedLat, err = optionalFloat(s, 1); err != nil {
		return out, err
	}
	if out.ExpectedErrorLon, out.HasExpectedLon, err = optionalFloat(s, 3); err != nil {
		return out, err
	}
	if out.ExpectedErrorAlt, out.HasExpectedAlt, err = optionalFloat(s, 5); err != nil {
		return out, err
	}

	// The unit letters are checked rather than skipped past: a receiver
	// reporting feet would otherwise have its figures silently treated as
	// metres, an error of more than a factor of three.
	for _, u := range []struct {
		index int
		what  string
	}{{2, "latitude error"}, {4, "longitude error"}, {6, "altitude error"}} {
		if !referenceOK(s.Field(u.index), "M") {
			return out, errMislabeled("GBS", u.what, s.Field(u.index))
		}
	}

	// A satellite id of 0 means no fault, which is how some receivers spell
	// "all clear". Treating it as satellite zero would report a bogus fault.
	if !s.Blank(7) {
		id, err := s.Int(7)
		if err != nil {
			return out, err
		}
		if id > 0 {
			out.FailedSatelliteID, out.HasFailedSatelliteID = id, true
		}
	}

	if out.ProbabilityOfMissedDetection, out.HasProbabilityOfMissedDetection, err = optionalFloat(s, 8); err != nil {
		return out, err
	}
	if out.EstimatedBias, out.HasEstimatedBias, err = optionalFloat(s, 9); err != nil {
		return out, err
	}
	if out.BiasStdDev, out.HasBiasStdDev, err = optionalFloat(s, 10); err != nil {
		return out, err
	}
	if id, ok := nmea.ParseSystemID(s.Field(11)); ok {
		out.SystemID, out.HasSystemID = id, true
	}
	if id, ok := nmea.ParseSignalID(s.Field(12)); ok {
		out.SignalID, out.HasSignalID = id, true
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence.
func (g GBS) NMEASupports() []nmea.Feature {
	if g.HasSystemID || g.HasSignalID {
		return nmea.FeaturesSystemAndSignal()
	}
	return nil
}

// ApplyFix folds the GBS into the fix state. GBS reports the expected error
// of a fix, not a fix, so it contributes accuracy and can invalidate a fix
// when integrity could not be established.
func (g GBS) ApplyFix(f *nmea.Fix) {
	if g.UTC.Available {
		f.TimeOfDay = g.UTC
	}
	if g.HasExpectedLat && g.HasExpectedLon {
		horizontal := hypot(g.ExpectedErrorLat, g.ExpectedErrorLon)
		f.HorizontalError, f.HasHorizontalError = horizontal, true
	}
	if g.HasExpectedAlt {
		f.VerticalError, f.HasVerticalError = g.ExpectedErrorAlt, true
	}
	if f.Valid && g.HasFailedSatelliteID {
		// A named satellite failure does not by itself make the position
		// unusable: the receiver may already have excluded that satellite
		// from the solution. It is recorded rather than acted on, because
		// discarding the fix here would be wrong more often than not.
		f.SatelliteFault = g.FailedSatelliteID
		f.HasSatelliteFault = true
	}
}
