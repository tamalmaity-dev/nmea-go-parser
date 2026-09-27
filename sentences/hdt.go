package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// HDT is the Heading, True sentence: a bare heading with nothing else, which
// makes it the sentence a program should reach for when it wants a heading and
// nothing more.
//
//	$GPHDT,274.07,T*03
//	$GNHDT,48.123,T*17
//
// Field layout:
//
//	0 heading   degrees true
//	1 reference T for true
//
// Some documentation describes HDT as deprecated, but that claim is
// uncorroborated and Actisense and others still translate it in current
// product sheets, so it is decoded as a normal sentence.
type HDT struct {
	Base
	Heading Heading
}

type hdt struct{}

func (hdt) Formatter() string { return "HDT" }

func (hdt) Decode(s nmea.Sentence) (any, error) {
	out := HDT{Base: newBase(s)}
	// The heading itself is the sentence, so its absence is a failure.
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	h, err := decodeHeading(s, "HDT")
	if err != nil {
		return out, err
	}
	if !h.HasDegrees {
		// The heading is the whole sentence, so a blank one leaves nothing to
		// deliver. It is reported as an empty required field rather than as a
		// field count problem: the field is present, it is the value that is
		// missing.
		return nil, errRequired("HDT", "heading")
	}
	out.Heading = h
	return out, nil
}

// ApplyFix folds the HDT into the fix state as the heading of the vessel.
// A true heading is the vessel's direction, not its direction of travel, so
// it is stored separately from the course over ground.
func (h HDT) ApplyFix(f *nmea.Fix) {
	if h.Heading.HasDegrees {
		f.HeadingDegrees, f.HasHeading = h.Heading.Degrees, true
		f.HeadingTrue = h.Heading.True
	}
}
