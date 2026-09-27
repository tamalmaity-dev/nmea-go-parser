package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// GRS is the GNSS Range Residuals sentence. It reports the difference
// between each satellite's pseudorange as measured and as expected, which is
// the raw data behind a receiver's error estimate.
//
//	$GPGRS,024603.00,1,-1.8,-2.7,0.3,,,,,,,*6C
//	$GPGRS,024603.00,1,-1.8,-2.7,0.3,0.0,0.0,0.0,0.0,0.0,0.0,0.0*40
//
// Field layout:
//
//	0 UTC time     of the associated GGA fix
//	1 mode         0 residuals used in the fix, 1 calculated after it
//	2-13 residuals in metres for satellites 1 to 12, blank when unused
//	14 system id   NMEA 4.10 and later, optional
//	15 signal id   NMEA 4.10 and later, optional
//
// The residuals are positional, not tagged: field 2 belongs to satellite 1,
// field 3 to satellite 2, and so on. The receiver's GSV is what says which
// satellite is which, and a caller combining the two must match on the
// sentence pair rather than assume. Mixing a GRS from one epoch with a GSV
// from another produces residuals attributed to the wrong satellites, which
// is why the timestamp is kept.
type GRS struct {
	Base
	// UTC is the time of the fix these residuals belong to.
	UTC nmea.TOD
	// Mode is 0 when the residuals were used to compute the fix and 1 when
	// they were computed afterwards. Mode 1 residuals describe a solution
	// that did not see them, which is what a receiver reports once it is
	// tracking more satellites than it needs.
	Mode    int
	HasMode bool
	// Residuals are the per-satellite values in metres, in satellite order,
	// with unused slots omitted.
	Residuals []float64
	// SatelliteNumbers pairs each residual with the satellite slot it came
	// from, which is its position in the GRS field range.
	SatelliteNumbers []int

	SystemID    nmea.SystemID
	HasSystemID bool
	SignalID    nmea.SignalDigit
	HasSignalID bool
}

// GRSResidualSlots is how many satellites a GRS sentence can describe, one
// residual each, matching the twelve slots of a GSA.
const GRSResidualSlots = 12

// MaxResidual returns the largest absolute residual in metres, and false when
// the sentence carried none. A residual much beyond a metre or two means
// either multipath or a bad satellite.
func (g *GRS) MaxResidual() (metres float64, ok bool) {
	for _, r := range g.Residuals {
		if a := abs(r); !ok || a > metres {
			metres, ok = a, true
		}
	}
	return metres, ok
}

type grs struct{}

func (grs) Formatter() string { return "GRS" }

func (grs) Decode(s nmea.Sentence) (any, error) {
	// Time and mode are required; the residuals themselves may all be blank.
	if !s.HasFields(2) {
		return nil, needFields(s, 2)
	}
	out := GRS{Base: newBase(s)}

	var err error
	if out.UTC, err = s.Time(0); err != nil {
		return out, err
	}
	if out.Mode, out.HasMode, err = optionalInt(s, 1); err != nil {
		return out, err
	}
	if out.HasMode && out.Mode != 0 && out.Mode != 1 {
		return out, errRange("GRS", "residual mode", float64(out.Mode), "0 or 1")
	}

	// Twelve fixed slots. The system and signal ids live at fields 14 and
	// 15, past the last slot, so a fixed twelve-field read cannot mistake a
	// system id of "1" for a residual.
	out.Residuals = make([]float64, 0, GRSResidualSlots)
	out.SatelliteNumbers = make([]int, 0, GRSResidualSlots)
	for i := 0; i < GRSResidualSlots; i++ {
		v, present, err := optionalFloat(s, 2+i)
		if err != nil {
			return out, err
		}
		if !present {
			continue
		}
		out.Residuals = append(out.Residuals, v)
		out.SatelliteNumbers = append(out.SatelliteNumbers, i+1)
	}

	if id, ok := nmea.ParseSystemID(s.Field(2 + GRSResidualSlots)); ok {
		out.SystemID, out.HasSystemID = id, true
	}
	if id, ok := nmea.ParseSignalID(s.Field(3 + GRSResidualSlots)); ok {
		out.SignalID, out.HasSignalID = id, true
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence.
func (g GRS) NMEASupports() []nmea.Feature {
	if g.HasSystemID || g.HasSignalID {
		return nmea.FeaturesSystemAndSignal()
	}
	return nil
}

// ApplyFix folds the GRS into the fix state. Residuals carry no position and
// no aggregate error figure, so nothing but the timestamp is folded in.
func (g GRS) ApplyFix(f *nmea.Fix) {
	if g.UTC.Available {
		f.TimeOfDay = g.UTC
	}
}
