package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// DBS is the Depth Below Surface sentence: the water depth measured from the
// waterline rather than from the transducer.
//
//	$SDDBS,7.8,f,2.4,M,1.3,F
//	$SDDBS,,f,22.5,M,,F
//
// Field layout:
//
//	0 depth, feet     f
//	1 depth, metres   M
//	2 depth, fathoms  F
//
// The difference from DBT is the transducer's height above the water, which is
// why the two disagree on the same boat. A keel-mounted transducer sits at the
// hull, so for it the two figures are the same measurement reported twice.
type DBS struct {
	DepthReading
}

type dbs struct{}

func (dbs) Formatter() string { return "DBS" }

func (dbs) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeDepthTriple(s)
	if err != nil {
		return body, err
	}
	return DBS{DepthReading: body}, nil
}

// ApplyFix folds the depth below the surface into the fix. It is kept apart
// from Fix.DepthMetres, which is the depth below the transducer, because the
// two differ by the transducer offset and reporting one as the other would be
// wrong by exactly that offset.
func (d DBS) ApplyFix(f *nmea.Fix) {
	if metres, ok := d.MetresOrConverted(); ok {
		f.DepthBelowSurface, f.HasSurfaceDepth = metres, true
	}
}
