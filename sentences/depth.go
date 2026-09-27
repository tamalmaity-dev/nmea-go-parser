package sentences

import (
	"fmt"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// The two depth conversions, in one place so DBT, DBS, DBK, and DPT cannot
// drift apart on them.
const (
	MetresPerFoot   = 0.3048
	MetresPerFathom = 1.8288
)

// DepthReading is the body shared by DBT, DBS, and DBK, whose field layouts are
// byte for byte identical: the same depth reported in three units.
//
//	$SDDBT,7.8,f,2.4,M,1.3,F
//	$SDDBS,,f,22.5,M,,F
//
// Field layout:
//
//	0 depth, feet     1 f
//	2 depth, metres   3 M
//	4 depth, fathoms  5 F
//
// The values and their unit letters alternate, so the three numbers are at
// fields 0, 2, and 4. Reading 0, 1, and 2 as though they were all values is the
// obvious mistake here, and it produces a sentence that fails on the unit letter
// "f" rather than anything resembling a depth.
//
// All three units are optional, and not merely because a receiver may be lazy
// with them. The documentation's own note is that real sensors often report only
// one, so a sentence carrying just metres is normal rather than truncated, which
// is why MetresOrConverted exists: a program almost always wants metres,
// whichever unit arrived.
//
// The three sentences differ only in what the depth is measured from. DBT is
// below the transducer, DBS below the surface, and DBK below the keel. DBK is
// marked obsolete in the standard in favour of DPT, but it is still emitted and
// is decoded here for the receivers that send it.
type DepthReading struct {
	Base
	// Metres is the depth in metres, the unit the other two convert to.
	Metres    float64
	HasMetres bool
	// Feet and Fathoms are the same depth as the receiver sent it.
	Feet       float64
	HasFeet    bool
	Fathoms    float64
	HasFathoms bool
}

// MetresOrConverted returns the depth in metres, converting from whichever
// unit the receiver actually sent, and false when it sent none.
//
// Metres is preferred when present because it is the receiver's own figure
// rather than one this library derived. A receiver that rounds its metre
// reading differently from its foot reading would otherwise be second-guessed.
func (d DepthReading) MetresOrConverted() (metres float64, ok bool) {
	switch {
	case d.HasMetres:
		return d.Metres, true
	case d.HasFeet:
		return d.Feet * MetresPerFoot, true
	case d.HasFathoms:
		return d.Fathoms * MetresPerFathom, true
	default:
		return 0, false
	}
}

// decodeDepthTriple reads the shared body. The three sentences have the same
// layout, so there is no reason to keep three copies of the field logic and
// three chances to get the indices wrong.
func decodeDepthTriple(s nmea.Sentence) (DepthReading, error) {
	var out DepthReading
	// The sentence is a whole number of value/unit pairs. A dangling value
	// with no unit letter is rejected rather than assumed: "7.8" alone is
	// ambiguous between feet and metres, and guessing wrong reports a depth
	// that is a factor of 3.3 out, which on a shallow bank is the difference
	// between scraping and clearing it.
	if n := len(s.Fields); n < 2 || n%2 != 0 {
		return out, fmt.Errorf(
			"sentences: %s has %d fields, want a whole number of depth/unit pairs: %w",
			s.Address(), n, nmea.ErrFieldCount)
	}
	out.Base = newBase(s)

	var err error
	if out.Feet, out.HasFeet, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	// The unit letters at fields 1, 3, and 5 are deliberately not read: the
	// format fixes them, so a receiver that labels a foot reading "M" has a
	// firmware bug rather than a unit worth honouring. Validating them would
	// reject real hardware over a cosmetic problem, and dropping a good depth
	// reading to catch a mislabelled letter is the wrong trade.
	if out.Metres, out.HasMetres, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.Fathoms, out.HasFathoms, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	return out, nil
}
