package sentences

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
)

// Decoder tests for this sentence. The TestPublished function below checks
// every field against the example sentence in the NMEA documentation; these
// tests cover the behaviour the published example does not reach.

func TestRTE(t *testing.T) {
	// Field 3 is the route ID, not a waypoint. Treating it as one puts a
	// bogus entry at the head of every route.
	r := decodeOne[RTE](t, "$GPRTE,2,1,c,0,PBRCPK,PBRTO,PTBR,PPBR*43")

	if r.TotalMessages != 2 || r.MessageNumber != 1 {
		t.Errorf("message %d of %d, want 1 of 2", r.MessageNumber, r.TotalMessages)
	}
	if r.Mode != ModeComplete {
		t.Errorf("Mode = %v, want complete", r.Mode)
	}
	if r.RouteID != "0" {
		t.Errorf("RouteID = %q, want 0", r.RouteID)
	}
	want := []string{"PBRCPK", "PBRTO", "PTBR", "PPBR"}
	if len(r.Waypoints) != len(want) {
		t.Fatalf("Waypoints = %v, want %v", r.Waypoints, want)
	}
	for i := range want {
		if r.Waypoints[i] != want[i] {
			t.Errorf("Waypoints[%d] = %q, want %q", i, r.Waypoints[i], want[i])
		}
	}
	if r.IsRequest() {
		t.Error("IsRequest = true for a route report, want false")
	}
	if r.CycleComplete() {
		t.Error("CycleComplete = true for the first of two sentences")
	}
}
func TestRTEWorkingMode(t *testing.T) {
	// The standard has two mode codes: c for complete, w for working. There
	// is no 'r' or 'p'.
	r := decodeOne[RTE](t, "$GPRTE,1,1,w,0*13")
	if r.Mode != ModeWorking {
		t.Errorf("Mode = %v, want working", r.Mode)
	}
	if r.IsRequest() {
		t.Error("IsRequest = true for a working-route report, want false")
	}
}
func TestRTEEncodeRespectsSentenceLimit(t *testing.T) {
	// A long route must be split so that no sentence exceeds the standard's
	// 82-byte limit, and every piece must be independently valid and
	// reassemble to the original order.
	long := []string{}
	for i := 0; i < 12; i++ {
		long = append(long, fmt.Sprintf("WAYPOINT%02d", i))
	}
	r := &RTE{Mode: ModeComplete, RouteID: "0", Waypoints: long}

	parts := r.Encode("GP")
	if len(parts) < 2 {
		t.Fatalf("got %d sentences for %d waypoints, want the route split",
			len(parts), len(long))
	}

	var rejoined []string
	for i, part := range parts {
		if len(part) > fault.MaxSentenceLength {
			t.Errorf("sentence %d is %d bytes, over the %d limit: %s",
				i, len(part), fault.MaxSentenceLength, part)
		}
		if err := nmea.Validate(part); err != nil {
			t.Errorf("Validate(sentence %d) = %v, want nil", i, err)
		}
		decoded := decodeOne[RTE](t, part)
		if decoded.MessageNumber != i+1 || decoded.TotalMessages != len(parts) {
			t.Errorf("sentence %d is numbered %d of %d, want %d of %d",
				i, decoded.MessageNumber, decoded.TotalMessages, i+1, len(parts))
		}
		if decoded.RouteID != "0" {
			t.Errorf("sentence %d has RouteID %q, want 0", i, decoded.RouteID)
		}
		rejoined = append(rejoined, decoded.Waypoints...)
	}

	if len(rejoined) != len(long) {
		t.Fatalf("reassembled %d waypoints, want %d", len(rejoined), len(long))
	}
	for i := range long {
		if rejoined[i] != long[i] {
			t.Errorf("waypoint %d = %q, want %q", i, rejoined[i], long[i])
		}
	}
}
func TestRTEEncodeShortRouteStaysOnOneSentence(t *testing.T) {
	// A route that already fits must not be split needlessly, or a receiver
	// would see a two-sentence cycle where one was sent.
	r := &RTE{Mode: ModeComplete, RouteID: "0", Waypoints: []string{"A", "B", "C"}}
	parts := r.Encode("GP")
	if len(parts) != 1 {
		t.Fatalf("got %d sentences for a 3-waypoint route, want 1", len(parts))
	}
	decoded := decodeOne[RTE](t, parts[0])
	if len(decoded.Waypoints) != 3 {
		t.Errorf("got %d waypoints, want 3", len(decoded.Waypoints))
	}
}
func TestFormattersList(t *testing.T) {
	got := Formatters()
	if len(got) != len(all) {
		t.Errorf("Formatters returned %d entries, want %d", len(got), len(all))
	}
	// The list must be sorted, since it is presented to a user.
	for i := 1; i < len(got); i++ {
		if got[i-1] > got[i] {
			t.Errorf("Formatters is not sorted at index %d: %q before %q", i, got[i-1], got[i])
		}
	}
}
func TestMagneticHeadingNeedsVariation(t *testing.T) {
	// A magnetic heading with no variation available must not be reported as
	// a true one: the difference is the local variation, which can exceed 20
	// degrees.
	h := Heading{Degrees: 45, HasDegrees: true, True: false}
	if _, ok := h.Bearing(0, false); ok {
		t.Error("a magnetic heading converted without variation")
	}
	d, ok := h.Bearing(-10, true)
	if !ok || d != 35 {
		t.Errorf("Bearing = %v, %v; want 35, true", d, ok)
	}
}

// Published-example checks: the field layout is verified against the
// example sentence printed in the NMEA documentation.

// TestPublishedRTE verifies that field 3 is the route ID and not a
// waypoint, and that the mode codes are c and w.
func TestPublishedRTE(t *testing.T) {
	// Published example.
	r := published[RTE](t, "GPRTE,1,1,c,0")

	// Field 0: one sentence. Field 1: it is sentence one.
	if r.TotalMessages != 1 || r.MessageNumber != 1 {
		t.Errorf("fields 0 and 1 = %d of %d, want 1 of 1",
			r.MessageNumber, r.TotalMessages)
	}
	// Field 2: the mode, c for a complete route.
	if r.Mode != ModeComplete {
		t.Errorf("field 2 mode = %v, want complete", r.Mode)
	}
	// Field 3: the route ID. Reading it as a waypoint would report a route
	// with a waypoint called "0" in it.
	if r.RouteID != "0" {
		t.Errorf("field 3 route ID = %q, want 0", r.RouteID)
	}
	if len(r.Waypoints) != 0 {
		t.Errorf("Waypoints = %v, want none for this example", r.Waypoints)
	}
	if r.IsRequest() {
		t.Error("IsRequest = true for a route report, want false")
	}

	// A working route uses w, the only other mode the standard defines.
	working := published[RTE](t, "GPRTE,1,1,w,0")
	if working.Mode != ModeWorking {
		t.Errorf("mode = %v, want working", working.Mode)
	}

	// Waypoints begin at field 4.
	full := published[RTE](t, "GPRTE,1,1,c,0,ALPHA,BRAVO,CHARLIE")
	if len(full.Waypoints) != 3 {
		t.Fatalf("Waypoints = %v, want three", full.Waypoints)
	}
	for i, want := range []string{"ALPHA", "BRAVO", "CHARLIE"} {
		if full.Waypoints[i] != want {
			t.Errorf("waypoint %d = %q, want %q", i, full.Waypoints[i], want)
		}
	}
}

// TestPublishedRTEEncodePreservesFieldOrder verifies the encoder emits
// total-then-number, which is the order the standard requires and the
// reverse of the one a receiver rejects.
func TestPublishedRTEEncodePreservesFieldOrder(t *testing.T) {
	r := &RTE{Mode: ModeComplete, RouteID: "0", Waypoints: []string{"ALPHA", "BRAVO"}}
	parts := r.Encode("GP")
	if len(parts) != 1 {
		t.Fatalf("got %d sentences, want 1", len(parts))
	}
	decoded := published[RTE](t, strings.TrimSuffix(strings.TrimPrefix(parts[0], "$"), "*"+
		parts[0][len(parts[0])-2:]))
	if decoded.MessageNumber != 1 || decoded.TotalMessages != 1 {
		t.Errorf("re-decoded as %d of %d, want 1 of 1",
			decoded.MessageNumber, decoded.TotalMessages)
	}
}
