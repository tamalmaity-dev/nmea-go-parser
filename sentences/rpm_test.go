package sentences

import (
	"strings"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// RPM decoder tests.

// TestPublishedRPM verifies the revolutions layout: source, number, speed,
// pitch, status.
func TestPublishedRPM(t *testing.T) {
	r := published[RPM](t, "IIRPM,S,1,1200.0,10.5,A")

	// Field 0: the source. It decides what the numbers mean, so it is checked
	// first.
	if r.Source != "S" || !r.IsShaft() {
		t.Errorf("field 0 source = %q, want S (shaft)", r.Source)
	}
	if r.IsEngine() {
		t.Error("IsEngine = true for a shaft sentence")
	}
	// Field 1: the engine or shaft number.
	if !r.HasNumber || r.Number != 1 {
		t.Errorf("field 1 number = %v (present %v), want 1", r.Number, r.HasNumber)
	}
	// Field 2: revolutions per minute.
	if !r.HasSpeed || r.Speed != 1200 {
		t.Errorf("field 2 speed = %v (present %v), want 1200", r.Speed, r.HasSpeed)
	}
	// Field 3: pitch, percent of maximum.
	if !r.HasPitch || r.Pitch != 10.5 {
		t.Errorf("field 3 pitch = %v (present %v), want 10.5", r.Pitch, r.HasPitch)
	}
	if r.Astern {
		t.Error("Astern = true for a positive pitch")
	}
	// Field 4: the status.
	if r.Status != 'A' {
		t.Errorf("field 4 status = %q, want A", r.Status)
	}
}

// TestRPMEngineSource covers the other source, which is the case that matters
// most in practice: a shaft and its engine turn at different speeds.
func TestRPMEngineSource(t *testing.T) {
	r := published[RPM](t, "IIRPM,E,2,850.0,,V")
	if r.Source != "E" || !r.IsEngine() {
		t.Errorf("source = %q, want E (engine)", r.Source)
	}
	if r.IsShaft() {
		t.Error("IsShaft = true for an engine sentence")
	}
	if !r.HasSpeed || r.Speed != 850 {
		t.Errorf("speed = %v, want 850", r.Speed)
	}
	// A blank pitch is a normal state, not a missing measurement.
	if r.HasPitch {
		t.Error("HasPitch = true for a blank pitch field")
	}
	if r.Status != 'V' {
		t.Errorf("status = %q, want V: an invalid reading still arrives with numbers", r.Status)
	}
}

// TestRPMAsternIsASignNotAPitch is the detail that gets lost. A negative pitch
// is a direction, not a small negative trim, so the magnitude and the direction
// are reported separately.
func TestRPMAsternIsASignNotAPitch(t *testing.T) {
	r := published[RPM](t, "IIRPM,E,1,800.0,-10.5,A")
	if !r.Astern {
		t.Error("Astern = false for a negative pitch")
	}
	// The magnitude is reported unsigned, because that is what a display wants.
	if r.Pitch != 10.5 {
		t.Errorf("Pitch = %v, want 10.5: the sign carries the direction, not the trim", r.Pitch)
	}
	if got := r.String(); !strings.Contains(got, "astern") {
		t.Errorf("String = %q, want it to say astern", got)
	}
}

// TestRPMRejectsUnknownSource covers the source validation. A mislabelled
// source is not cosmetic: it decides which reading a number belongs to, and a
// boat whose shaft is not at its engine's speed would report a figure off by
// the gear ratio.
func TestRPMRejectsUnknownSource(t *testing.T) {
	if err := publishedErr(t, "IIRPM,X,1,1200.0,10.5,A"); err == nil {
		t.Error("decoding an RPM with source X succeeded, want an error")
	}
	if err := publishedErr(t, "IIRPM,,1,1200.0,10.5,A"); err == nil {
		t.Error("decoding an RPM with a blank source succeeded, want an error")
	}
	if err := publishedErr(t, "IIRPM"); err == nil {
		t.Error("decoding an RPM with no fields succeeded, want an error")
	}
}

// TestRPMSpeedOnly covers a receiver reporting speed with no pitch, which is
// what an engine monitor sends when it cannot read the pitchpot.
func TestRPMSpeedOnly(t *testing.T) {
	r := published[RPM](t, "IIRPM,E,1,1500.0,,A")
	if !r.HasSpeed || r.Speed != 1500 {
		t.Errorf("speed = %v (present %v), want 1500", r.Speed, r.HasSpeed)
	}
	if r.HasPitch {
		t.Error("HasPitch = true for a blank pitch")
	}
}

// TestRPMLabels covers the display name, which is what tells a multi-engine
// readout which figure is which.
func TestRPMLabels(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"IIRPM,S,1,1200.0,10.5,A", "shaft 1"},
		{"IIRPM,E,2,850.0,10.5,A", "engine 2"},
	} {
		if got := published[RPM](t, tc.body).Label(); got != tc.want {
			t.Errorf("Label = %q, want %q", got, tc.want)
		}
	}
	if got := published[RPM](t, "IIRPM,S,,1200.0,10.5,A").Label(); got != "shaft" {
		t.Errorf("Label = %q, want %q for a sentence with no number", got, "shaft")
	}
}

// TestRPMDoesNotTouchTheFix checks that engine data is not an observation.
func TestRPMDoesNotTouchTheFix(t *testing.T) {
	var v any = published[RPM](t, "IIRPM,S,1,1200.0,10.5,A")
	if _, ok := v.(nmea.FixContributor); ok {
		t.Error("RPM implements FixContributor, but it carries no positional data")
	}
}
