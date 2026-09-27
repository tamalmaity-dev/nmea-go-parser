package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// DTM is the Datum Reference sentence. It names the geodetic datum the
// receiver's position is expressed in, which matters whenever positions are
// compared with a chart or a survey: a position on WGS84 and the same point
// on a local datum can differ by hundreds of metres.
//
//	$GPDTM,W84,C
//	$GPDTM,W84,,0.0,N,0.0,E,0.0,W84
//
// Field layout:
//
//	0 local datum code       e.g. W84
//	1 local datum subcode    may be blank
//	2 latitude offset        arc minutes
//	3 N or S
//	4 longitude offset       arc minutes
//	5 E or W
//	6 altitude offset        metres
//	7 datum name             e.g. W84
//
// The only form reliably found in the field is the short two-field one, so
// every field after the code is optional and the offsets are presence-flagged.
// A receiver that sends only "W84" is stating a datum with no shift, which is
// the normal case.
type DTM struct {
	Base
	// LocalDatum is the datum code from field 0, e.g. "W84".
	LocalDatum string
	// Subcode is the optional subcode, frequently "C" for a chart datum
	// reference.
	Subcode string
	// LatOffset and LonOffset are in arc minutes, which is the unit the
	// standard uses for the shift to the local datum.
	LatOffset    float64
	HasLatOffset bool
	LonOffset    float64
	HasLonOffset bool
	// AltOffset is in metres.
	AltOffset    float64
	HasAltOffset bool
	// DatumName is the human-readable name, which often repeats the code.
	DatumName string
}

// WGS84 is the datum code for the World Geodetic System 1984, the default
// for GNSS and for practically all modern charts.
const WGS84 = "W84"

// Metres returns the horizontal shift to the local datum in metres, and false
// when the sentence carried no offset. The two axis shifts are combined as a
// vector, which is what a caller comparing against a chart needs.
func (d *DTM) Metres() (metres float64, ok bool) {
	if !d.HasLatOffset && !d.HasLonOffset {
		return 0, false
	}
	// A minute of latitude is about 1852 m; a minute of longitude is the
	// same scaled by the cosine of the latitude, which the sentence does not
	// carry. The un-scaled value is returned, and documented as approximate,
	// because the alternative is inventing a latitude.
	return hypot(d.LatOffset, d.LonOffset) * 1852.0, true
}

type dtm struct{}

func (dtm) Formatter() string { return "DTM" }

func (dtm) Decode(s nmea.Sentence) (any, error) {
	// The datum code is the whole point of the sentence, so it is required.
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := DTM{
		LocalDatum: text(s.Field(0)),
		Subcode:    text(s.Field(1)),
	}
	if out.LocalDatum == "" {
		// A blank code names no datum, which defeats the sentence.
		return out, nil
	}

	var err error
	if out.LatOffset, out.HasLatOffset, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.HasLatOffset && s.Field(3) == "S" {
		out.LatOffset = -out.LatOffset
	}
	if out.LonOffset, out.HasLonOffset, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.HasLonOffset && s.Field(5) == "W" {
		out.LonOffset = -out.LonOffset
	}
	if out.AltOffset, out.HasAltOffset, err = optionalFloat(s, 6); err != nil {
		return out, err
	}
	out.DatumName = text(s.Field(7))
	return out, nil
}
