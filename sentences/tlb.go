package sentences

import (
	"fmt"
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TLB is the Target Label sentence: names for tracked targets, as target number
// and label pairs repeated until the sentence ends.
//
//	$GPTLB,1,WHALE,2,TARGET2,3,TARGET3
//
// Field layout, repeated:
//
//	0 target number, 0 to 99
//	1 the label assigned to it
//
// TLB is the variable-length sentence of the trawl group, for the same reason
// XDR is: how many targets a tracker is following is the vessel's business, not
// the format's. A sentence with a dangling number and no label is rejected
// rather than half-read, because a label attached to the wrong target is worse
// than no label at all.
//
// The number refers back to the TTM and TLL sentences for the same target, and
// is the only way to tell which motion and position belong to which name.
type TLB struct {
	Base
	// Labels are the target number and name pairs, in the order sent.
	Labels []TargetLabel
}

// TargetLabel is one target's assigned name.
type TargetLabel struct {
	Number    int
	HasNumber bool
	Name      string
}

// Label returns the name assigned to a target number, and false when the
// sentence did not carry one. This is the lookup a display does per target.
func (t TLB) Label(number int) (string, bool) {
	for _, l := range t.Labels {
		if l.HasNumber && l.Number == number {
			return l.Name, true
		}
	}
	return "", false
}

// Numbers returns the target numbers the sentence named, which is how a program
// knows how far a tracker's labelling has got.
func (t TLB) Numbers() []int {
	out := make([]int, 0, len(t.Labels))
	for _, l := range t.Labels {
		if l.HasNumber {
			out = append(out, l.Number)
		}
	}
	return out
}

// String renders the labels for a log line.
func (t TLB) String() string {
	parts := make([]string, 0, len(t.Labels))
	for _, l := range t.Labels {
		parts = append(parts, fmt.Sprintf("%d=%s", l.Number, l.Name))
	}
	return strings.Join(parts, " ")
}

type tlb struct{}

func (tlb) Formatter() string { return "TLB" }

func (tlb) Decode(s nmea.Sentence) (any, error) {
	out := TLB{Base: newBase(s)}

	// The payload is a whole number of number and label pairs. A dangling
	// number with no label is rejected: a name attached to nothing is worse
	// than a name missing.
	if n := len(s.Fields) % 2; n != 0 {
		return out, fmt.Errorf(
			"sentences: TLB has %d fields, which is not a whole number of target/label pairs: %w",
			len(s.Fields), nmea.ErrFieldCount)
	}
	if len(s.Fields) == 0 {
		return out, needFields(s, 2)
	}

	out.Labels = make([]TargetLabel, 0, len(s.Fields)/2)
	for i := 0; i < len(s.Fields); i += 2 {
		var l TargetLabel
		var err error
		if l.Number, l.HasNumber, err = optionalInt(s, i); err != nil {
			return out, err
		}
		l.Name = text(s.Field(i + 1))
		out.Labels = append(out.Labels, l)
	}
	return out, nil
}
