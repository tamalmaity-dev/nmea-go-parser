package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VLW is the Distance Travelled through Water sentence. It reports cumulative
// and trip distances, and from NMEA 3.0 both the water figures and the
// over-the-ground figures.
//
//	$GPVLW,,N,,N,1234.5,N,2345.6,N
//
// Field layout:
//
//	0 total distance through water nautical miles
//	1 units                N
//	2 distance since reset  nautical miles
//	3 units                N
//	4 total distance over ground nautical miles, NMEA 3.0 and later
//	5 units                N
//	6 ground distance since reset nautical miles, NMEA 3.0 and later
//	7 units                N
//
// The cumulative figures are the reason a receiver keeps them: they survive a
// reboot, so a program can log mileage across a power cycle. Trip figures
// reset, usually when the user asks.
//
// The difference between the water and ground totals is distance run, which
// is how a program measures current set and drift without needing a separate
// water-speed input.
type VLW struct {
	Base
	// TotalWater and TripWater are through the water, in nautical miles.
	TotalWater    float64
	HasTotalWater bool
	TripWater     float64
	HasTripWater  bool
	// TotalGround and TripGround are over the ground, NMEA 3.0 and later.
	TotalGround    float64
	HasTotalGround bool
	TripGround     float64
	HasTripGround  bool
}

// Metres converts a nautical-mile distance to metres. The sentence has no
// metric figures at all, so every consumer of VLW needs this.
func Metres(nauticalMiles float64) float64 { return nauticalMiles * 1852 }

// Kilometres converts a nautical-mile distance to kilometres.
func Kilometres(nauticalMiles float64) float64 { return nauticalMiles * 1.852 }

// SetAndDrift returns the distance run between the water and ground totals,
// which is the accumulated effect of current and leeway over the whole trip.
func (v *VLW) SetAndDrift() (nauticalMiles float64, ok bool) {
	if !v.HasTotalWater || !v.HasTotalGround {
		return 0, false
	}
	return abs(v.TotalGround - v.TotalWater), true
}

type vlw struct{}

func (vlw) Formatter() string { return "VLW" }

func (vlw) Decode(s nmea.Sentence) (any, error) {
	// A receiver with no odometer emits VLW with every field blank, which is
	// valid, so only an entirely absent field list fails.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := VLW{Base: newBase(s)}

	var err error
	if out.TotalWater, out.HasTotalWater, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.TripWater, out.HasTripWater, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.TotalGround, out.HasTotalGround, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.TripGround, out.HasTripGround, err = optionalFloat(s, 6); err != nil {
		return out, err
	}

	// Each value is followed by a unit field. A receiver using a non-standard
	// unit letter would otherwise have its figures silently reinterpreted as
	// nautical miles, so the letters are checked.
	for _, u := range []struct {
		index int
		what  string
	}{{1, "water total"}, {3, "water trip"}, {5, "ground total"}, {7, "ground trip"}} {
		if !referenceOK(s.Field(u.index), "N") {
			return out, errMislabeled("VLW", u.what+" distance", s.Field(u.index))
		}
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: the ground distances arrived
// in NMEA 3.0.
func (v VLW) NMEASupports() []nmea.Feature {
	if v.HasTotalGround || v.HasTripGround {
		return nmea.FeatureOnly(nmea.FeatureGroundSpeed)
	}
	return nil
}

// ApplyFix folds the VLW into the fix state. Distance figures are not part of
// a position fix, so nothing is folded in.
func (v VLW) ApplyFix(f *nmea.Fix) {}
