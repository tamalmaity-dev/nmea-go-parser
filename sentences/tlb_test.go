package sentences

import (
	"strings"
	"testing"
)

// TLB decoder tests.

// TestPublishedTLB verifies the repeated target and label pairs.
func TestPublishedTLB(t *testing.T) {
	tl := published[TLB](t, "GPTLB,1,WHALE,2,TARGET2,3,TARGET3")

	if len(tl.Labels) != 3 {
		t.Fatalf("got %d labels, want 3: the payload is a repeating pair", len(tl.Labels))
	}
	for i, want := range []struct {
		number int
		name   string
	}{
		{1, "WHALE"}, {2, "TARGET2"}, {3, "TARGET3"},
	} {
		l := tl.Labels[i]
		if !l.HasNumber || l.Number != want.number {
			t.Errorf("label %d number = %v (present %v), want %d", i, l.Number, l.HasNumber, want.number)
		}
		if l.Name != want.name {
			t.Errorf("label %d name = %q, want %q", i, l.Name, want.name)
		}
	}
	if tl.DataType() != "TLB" {
		t.Errorf("DataType = %q, want TLB", tl.DataType())
	}
}

// TestTLBLookup covers the per-target lookup a display does, and the numbers
// list that says how far the labelling has got.
func TestTLBLookup(t *testing.T) {
	tl := published[TLB](t, "GPTLB,1,WHALE,2,TARGET2,3,TARGET3")

	if name, ok := tl.Label(2); !ok || name != "TARGET2" {
		t.Errorf("Label(2) = %q, %v; want TARGET2, true", name, ok)
	}
	if _, ok := tl.Label(9); ok {
		t.Error("Label(9) found something for a target the sentence did not name")
	}
	if got := tl.Numbers(); len(got) != 3 || got[0] != 1 || got[2] != 3 {
		t.Errorf("Numbers = %v, want [1 2 3]", got)
	}
	if got := tl.String(); !strings.Contains(got, "1=WHALE") {
		t.Errorf("String = %q, want it to pair numbers with names", got)
	}
}

// TestTLBBlankLabel covers a target the tracker has a number for but no name
// yet, which is a normal state while a track is being acquired.
func TestTLBBlankLabel(t *testing.T) {
	tl := published[TLB](t, "GPTLB,1,,2,TARGET2")
	if len(tl.Labels) != 2 {
		t.Fatalf("got %d labels, want 2", len(tl.Labels))
	}
	if tl.Labels[0].Name != "" {
		t.Errorf("label 0 name = %q, want empty", tl.Labels[0].Name)
	}
	if !tl.Labels[0].HasNumber || tl.Labels[0].Number != 1 {
		t.Error("the target number was lost for a blank label")
	}
	// An empty name is a name that happens to be empty, so the lookup finds it
	// rather than reporting the target as unknown.
	if name, ok := tl.Label(1); !ok || name != "" {
		t.Errorf("Label(1) = %q, %v; want an empty string, true", name, ok)
	}
}

// TestTLBRejectsDanglingNumber covers a sentence whose payload is not a whole
// number of pairs. A label attached to nothing is worse than a label missing,
// because it silently attaches to the wrong target.
func TestTLBRejectsDanglingNumber(t *testing.T) {
	// A trailing empty label is deliberately not in this list: it is a whole
	// second pair, with a target that has a number and no name yet.
	for _, body := range []string{"GPTLB,1", "GPTLB,1,WHALE,2"} {
		if err := publishedErr(t, body); err == nil {
			t.Errorf("decoding %q succeeded, want an error", body)
		}
	}
}

// TestTLBEmptyRejected covers a sentence with nothing in it.
func TestTLBEmptyRejected(t *testing.T) {
	if err := publishedErr(t, "GPTLB"); err == nil {
		t.Error("decoding a TLB with no fields succeeded, want an error")
	}
}
