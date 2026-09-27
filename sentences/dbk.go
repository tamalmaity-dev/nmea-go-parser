package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// DBK is the Depth Below Keel sentence, which reports the depth beneath the
// lowest point of the hull directly rather than making the caller compute it.
//
//	$SDDBK,7.8,f,2.4,M,1.3,F
//
// Field layout:
//
//	0 depth, feet     f
//	1 depth, metres   M
//	2 depth, fathoms  F
//
// DBK is marked obsolete in the standard, superseded by DPT, and receivers
// that support DPT rarely send it. It is decoded anyway: it costs one file, it
// is still on the wire from older equipment, and it carries the one figure a
// program cannot derive from the others without knowing the hull.
type DBK struct {
	DepthReading
}

type dbk struct{}

func (dbk) Formatter() string { return "DBK" }

func (dbk) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeDepthTriple(s)
	if err != nil {
		return body, err
	}
	return DBK{DepthReading: body}, nil
}

// ApplyFix folds the depth below the keel into the fix, which is the figure a
// navigation program actually needs: it is what decides whether the next
// grounding is going to happen.
func (d DBK) ApplyFix(f *nmea.Fix) {
	if metres, ok := d.MetresOrConverted(); ok {
		f.DepthBelowKeel, f.HasKeelDepth = metres, true
	}
}
