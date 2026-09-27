package sentences

import (
	"github.com/tamalmaity-dev/nmea-go-parser"
)

// Base carries what every decoded sentence has in common: the formatter, the
// talker, and the text it arrived as.
//
// Each sentence struct embeds it, which is what makes every decoded value
// satisfy nmea.DecodedSentence without thirty copies of the same four methods. A
// caller can therefore route on the type before switching:
//
//	if s.DataType() == nmea.TypeRMC {
//	    m := s.(nmea.RMC)   // a value, not a pointer
//	    ...
//	}
type Base struct {
	// Sentence is the framed sentence this value was decoded from. Its fields
	// are reachable for anything a decoder does not surface, which is the
	// escape hatch for a vendor extension this library does not model.
	Sentence nmea.Sentence
}

func newBase(s nmea.Sentence) Base { return Base{Sentence: s} }

// DataType returns the sentence formatter, one of the nmea.Type constants.
func (b Base) DataType() nmea.Type { return b.Sentence.Type }

// Talker returns the two-character source designator, e.g. "GP" or "GN".
func (b Base) Talker() string { return b.Sentence.Talker }

// Address returns the talker and formatter together, e.g. "GNGGA".
func (b Base) Address() string { return b.Sentence.Address() }

// Raw returns the sentence exactly as it arrived, checksum included.
func (b Base) Raw() string { return b.Sentence.Raw }

// ChecksumState reports whether the sentence's checksum verified, was absent,
// or was wrong.
func (b Base) ChecksumState() nmea.ChecksumStatus { return b.Sentence.ChecksumState }

// String returns the raw sentence, so a decoded value prints as its wire
// form in a log line rather than as a struct dump.
func (b Base) String() string { return b.Sentence.Raw }

// Constellation reports which satellite system the sentence came from, taken
// from the talker.
func (b Base) Constellation() nmea.Constellation {
	return nmea.TalkerConstellation(b.Sentence.Talker)
}

// LatLon is the position a sentence reported.
//
// The decimal degrees are the fields a program normally wants, and they are
// plain float64 so they can go straight into a map, a maths function, or
// nmea.FormatGPS. The raw form is kept alongside because a value that arrived
// as "01131.000" has to re-encode to exactly that, leading zero included, or
// a waypoint read out of one receiver and written to another is silently
// reformatted.
//
// A receiver searching for satellites sends the position blank, which is
// normal and not an error. HasPosition distinguishes that from a real
// position of 0.0, 0.0 in the Gulf of Guinea.
type LatLon struct {
	// Latitude and Longitude are signed decimal degrees: positive latitude
	// is north, positive longitude is east. They are zero when HasPosition is
	// false.
	Latitude  float64
	Longitude float64
	// HasPosition is true when the receiver supplied both halves.
	HasPosition bool
	// LatitudeRaw and LongitudeRaw are the values as they arrived, still in
	// ddmm.mmmm form, for exact re-encoding and for diagnostics.
	LatitudeRaw  nmea.Coordinate
	LongitudeRaw nmea.Coordinate
}

// latLonFrom converts a coordinate pair starting at field i, which holds a
// latitude at i and a longitude at i+2, each followed by its hemisphere
// letter.
func latLonFrom(s nmea.Sentence, i int) LatLon {
	var out LatLon
	out.LatitudeRaw, _ = s.Coordinate(i, nmea.AxisLatitude)
	out.LongitudeRaw, _ = s.Coordinate(i+2, nmea.AxisLongitude)
	out.HasPosition = fillDecimals(&out)
	return out
}

// latLonOptional is latLonFrom for a position a receiver may legitimately
// leave blank, such as a BWC whose waypoint is not stored. A blank field
// yields an empty LatLon and no error, rather than a parse failure.
func latLonOptional(s nmea.Sentence, i int) (LatLon, error) {
	if s.Blank(i) || s.Blank(i+2) {
		return LatLon{}, nil
	}
	return latLonFrom(s, i), nil
}

// fillDecimals converts the raw coordinates to signed decimal degrees and
// reports whether both were usable.
func fillDecimals(p *LatLon) bool {
	lat, err := p.LatitudeRaw.Decimal()
	if err != nil {
		return false
	}
	lon, err := p.LongitudeRaw.Decimal()
	if err != nil {
		return false
	}
	p.Latitude, p.Longitude = lat, lon
	return true
}

// applyPosition writes a LatLon into the fix, which is the only way a
// sentence should ever move the reported position. It reports false when the
// sentence carried no position, so a caller can leave the fix untouched.
func applyPosition(f *nmea.Fix, p LatLon) bool {
	if !p.HasPosition {
		return false
	}
	return f.SetPositionDecimal(p.Latitude, p.Longitude) == nil
}
