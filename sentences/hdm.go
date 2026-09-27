package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// HDM is the Heading, Magnetic sentence: the vessel's heading relative to
// magnetic north, as a magnetic compass reads it.
//
//	$HCHDM,123.4,M
//
// Field layout:
//
//	0 heading, degrees magnetic   1 M
//
// HDM is deprecated in the standard in favour of HDT, which carries a reference
// flag and so can express both norths in one sentence. It is decoded anyway
// because a great many magnetic compasses still emit it, and it is the only
// heading some older instruments can produce.
//
// The reference letter is fixed by the format and deliberately not validated.
// It is worth saying why: unlike a T/M reference elsewhere in this package,
// getting this one wrong cannot produce a plausible wrong number, because the
// sentence has only one field and no alternative reading of it. A compass that
// mislabels its own output is broken in a way this decoder cannot detect, and
// refusing to decode it would only hide the fault rather than fix it.
type HDM struct {
	Base
	HeadingDegrees float64
	HasHeading     bool
}

type hdm struct{}

func (hdm) Formatter() string { return "HDM" }

func (hdm) Decode(s nmea.Sentence) (any, error) {
	out := HDM{Base: newBase(s)}
	// A compass with no reading is a normal state, so one field is enough.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}
	var err error
	if out.HeadingDegrees, out.HasHeading, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	// A magnetic heading outside 0 to 360 is a compass fault, and folding it
	// into the fix would leave a vessel with a heading of 400 degrees, which
	// is worse than no heading at all.
	if out.HasHeading && (out.HeadingDegrees < 0 || out.HeadingDegrees > 360) {
		return out, errRange("HDM", "heading", out.HeadingDegrees, "0..360")
	}
	return out, nil
}

// ApplyFix folds the magnetic heading into the fix.
//
// HeadingTrue is left false, which is the whole point of keeping the two apart:
// a magnetic heading differs from a true one by the local variation, and
// reporting it as true would put the vessel tens of degrees off in the
// direction that matters most, silently.
func (h HDM) ApplyFix(f *nmea.Fix) {
	if !h.HasHeading {
		return
	}
	f.HeadingDegrees, f.HasHeading = h.HeadingDegrees, true
	f.HeadingTrue = false
}
