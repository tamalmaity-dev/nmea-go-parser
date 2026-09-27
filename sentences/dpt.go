package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// DPT is the Depth of Water sentence: the depth under the transducer, the
// transducer's offset from the waterline, and the range scale in use.
//
//	$INDPT,2.3,0.0
//	$SDDBT,7.8,f,2.4,M,1.3,F
//
// Field layout:
//
//	0 depth        metres below the transducer
//	1 offset       metres from the transducer to the waterline
//	2 range scale  metres, NMEA 3.0 and later, optional
//
// The sign of the offset is the detail that matters most, and it is
// documented two different ways in the wild. The convention used here is the
// one that makes depth below the keel computable without a second guess:
// a positive offset means the transducer is above the waterline, so keel
// depth is offset minus depth, and a negative offset means the transducer is
// below the waterline, so keel depth is offset plus depth.
//
// Getting this backwards reports a depth that is wrong by twice the offset,
// which on a shallow-draft vessel is the difference between safe and
// aground.
type DPT struct {
	Base
	// Depth is metres of water below the transducer.
	Depth    float64
	HasDepth bool
	// Offset is metres from the transducer to the waterline, positive above
	// it and negative below.
	Offset    float64
	HasOffset bool
	// RangeScale is the maximum range the sounder is set to, metres.
	RangeScale    float64
	HasRangeScale bool
}

// DepthBelowKeel returns the water depth beneath the lowest point of the
// hull, and false when either the depth or the offset is missing.
//
// It also reports false when the result is negative, which means the vessel
// is aground: returning a negative depth as though it were a measurement
// would let a caller treat a grounding as a very shallow passage.
func (d *DPT) DepthBelowKeel() (metres float64, ok bool) {
	if !d.HasDepth || !d.HasOffset {
		return 0, false
	}
	if d.Offset > 0 {
		// Transducer above the waterline: the hull extends below it.
		metres = d.Offset - d.Depth
	} else {
		// Transducer below the waterline: the offset deepens the reading.
		metres = d.Offset + d.Depth
	}
	if metres < 0 {
		return metres, false
	}
	return metres, true
}

// Fathoms converts a depth in metres to fathoms, the unit a depth sounder
// display will show. It lives here rather than in depth.go because DPT is the
// sentence a caller reaches for when converting a depth.
func Fathoms(metres float64) float64 { return metres / MetresPerFathom }

type dpt struct{}

func (dpt) Formatter() string { return "DPT" }

func (dpt) Decode(s nmea.Sentence) (any, error) {
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := DPT{Base: newBase(s)}

	var err error
	if out.Depth, out.HasDepth, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.Offset, out.HasOffset, err = optionalFloat(s, 1); err != nil {
		return out, err
	}
	if out.RangeScale, out.HasRangeScale, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: the range scale arrived in
// NMEA 3.0.
func (d DPT) NMEASupports() []nmea.Feature {
	if d.HasRangeScale {
		return nmea.FeatureOnly(nmea.FeatureRangeScale)
	}
	return nil
}

// ApplyFix folds the depth into the fix state.
//
// DPT is the preferred depth sentence and the only one that can reach a
// depth-below-keel figure, because it is the only one carrying the transducer
// offset. A receiver that sends both DPT and DBT will produce the same
// depth, which is the correct outcome rather than a conflict.
func (d DPT) ApplyFix(f *nmea.Fix) {
	if d.HasDepth {
		f.DepthMetres, f.HasDepth = d.Depth, true
	}
	if keel, ok := d.DepthBelowKeel(); ok {
		f.DepthBelowKeel, f.HasKeelDepth = keel, true
	}
}
