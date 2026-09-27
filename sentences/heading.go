package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// Heading is the common part of the heading sentences. HDT, THS, and HDG all
// report a heading, differing only in reference and in how much they say
// about its provenance, so the shared parts live here rather than being
// copied three times.
type Heading struct {
	// Degrees is the heading. True north is 0, and the value increases
	// clockwise.
	Degrees    float64
	HasDegrees bool
	// True reports whether the heading is referenced to true north. A false
	// value with HasDegrees set is a magnetic heading, which differs from a
	// true one by the local variation and must not be treated as the same
	// number.
	True bool
	// Mode is the provenance indicator, where the sentence carries one.
	Mode nmea.StatusFlag
}

// Bearing returns the heading in degrees true, converting a magnetic heading
// with the supplied variation in degrees, east positive.
//
// It reports false when the heading is already true, because there is nothing
// to convert, and false when no variation is available for a magnetic one,
// rather than silently returning a magnetic figure as though it were true.
func (h Heading) Bearing(variation float64, hasVariation bool) (deg float64, ok bool) {
	if !h.HasDegrees {
		return 0, false
	}
	if h.True {
		return h.Degrees, true
	}
	if !hasVariation {
		return 0, false
	}
	// Magnetic is measured from magnetic north, so adding the easterly
	// variation gives the same direction referenced to true north.
	deg = h.Degrees + variation
	for deg >= 360 {
		deg -= 360
	}
	for deg < 0 {
		deg += 360
	}
	return deg, true
}

func decodeHeading(s nmea.Sentence, formatter string) (Heading, error) {
	var h Heading
	v, present, err := optionalFloat(s, 0)
	if err != nil {
		return h, err
	}
	if !present {
		return h, nil
	}
	if v < 0 || v > 360 {
		return h, errRange(formatter, "heading", v, "0..360")
	}
	h.Degrees, h.HasDegrees = v, true

	// The reference flag is what stops a magnetic heading being read as a
	// true one, a 10 to 20 degree error that is otherwise invisible.
	switch s.Field(1) {
	case "T", "t":
		h.True = true
	case "M", "m":
		h.True = false
	case "":
		// Absent reference: the value is present but unlabelled. Leave True
		// false and let the caller decide, since assuming true would be the
		// more dangerous of the two guesses.
	default:
		return h, errMislabeled(formatter, "heading", s.Field(1))
	}
	return h, nil
}
