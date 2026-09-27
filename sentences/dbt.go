package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// DBT is the Depth Below Transducer sentence: the water depth measured by a SONAR.
// AND reported in feet, metres, and fathoms at once.

// --------------------------------------------------------------------------------------
//
//	$SDDBT,7.8,f,2.4,M,1.3,F
//	$SDDBT,,f,22.5,M,,F
//
// ---------------------------------------------------------------------------------------

// Field layout:
//
//	0 depth, feet     f
//	1 depth, metres   M
//	2 depth, fathoms  F
//
// DBT and DPT carry the same measurement. DPT adds the transducer offset, which
// is what makes a depth below the keel computable, so a program that has DPT
// should prefer it. A receiver that emits both produces the same depth twice,
// which is the correct outcome rather than a conflict.

type DBT struct {
	DepthReading
}

type dbt struct{}

func (dbt) Formatter() string { return "DBT" }

func (dbt) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeDepthTriple(s)
	if err != nil {
		return body, err
	}
	return DBT{DepthReading: body}, nil
}

// ApplyFix folds the depth below the transducer into the fix.
func (d DBT) ApplyFix(f *nmea.Fix) {
	if metres, ok := d.MetresOrConverted(); ok {
		f.DepthMetres, f.HasDepth = metres, true
	}
}
