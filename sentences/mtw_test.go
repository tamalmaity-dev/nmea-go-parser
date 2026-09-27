package sentences

import (
	"testing"
)

// TestPublishedMTW verifies the water temperature layout against the example
// printed in the NMEA documentation:
//
//	$INMTW,17.9,C*1B
func TestPublishedMTW(t *testing.T) {
	m := published[MTW](t, "INMTW,17.9,C")

	if !m.HasCelsius {
		t.Fatal("HasCelsius = false for a sentence carrying a temperature")
	}
	if !near(m.Celsius, 17.9, 1e-9) {
		t.Errorf("field 0 temperature = %v, want 17.9", m.Celsius)
	}
	if m.Unit != "C" {
		t.Errorf("field 1 unit = %q, want C", m.Unit)
	}

	// The conversion is the only reason to keep Celsius, so it is checked
	// rather than left to the caller.
	f, ok := m.Fahrenheit()
	if !ok {
		t.Fatal("Fahrenheit reported no value for a sentence carrying a temperature")
	}
	if !near(f, 17.9*9/5+32, 1e-9) {
		t.Errorf("Fahrenheit = %v, want %v", f, 17.9*9/5+32)
	}
}

// TestMTWBlankReading covers the transducer that is present but not reporting.
// The field is blank, which is not a malformed sentence, so it decodes with
// HasCelsius false and Fahrenheit reporting no value.
func TestMTWBlankReading(t *testing.T) {
	m := published[MTW](t, "INMTW,,")
	if m.HasCelsius {
		t.Error("HasCelsius = true for a blank temperature field")
	}
	if _, ok := m.Fahrenheit(); ok {
		t.Error("Fahrenheit reported a value for a blank temperature field")
	}
}

// TestMTWRejectsFahrenheitUnit checks that a receiver reporting degrees
// Fahrenheit is caught. C is the only unit the format defines, so a value
// carrying F is either a different sentence or a mislabelled reading, and
// silently treating it as Celsius would be a 32 degree error.
func TestMTWRejectsFahrenheitUnit(t *testing.T) {
	if err := publishedErr(t, "INMTW,64.2,F"); err == nil {
		t.Error("decoding an MTW with an F unit succeeded, want an error")
	}
}
