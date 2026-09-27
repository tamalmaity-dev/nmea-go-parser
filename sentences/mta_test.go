package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// MTA decoder tests.

// TestPublishedMTA verifies the air temperature layout.
func TestPublishedMTA(t *testing.T) {
	m := published[MTA](t, "IIMTA,13.3,C")

	if !m.HasCelsius || m.Celsius != 13.3 {
		t.Errorf("field 0 temperature = %v (present %v), want 13.3", m.Celsius, m.HasCelsius)
	}
	if m.Unit != "C" {
		t.Errorf("field 1 unit = %q, want C", m.Unit)
	}
	if m.DataType() != "MTA" {
		t.Errorf("DataType = %q, want MTA", m.DataType())
	}

	// The conversion is the only reason to keep Celsius, so it is checked.
	f, ok := m.Fahrenheit()
	if !ok {
		t.Fatal("Fahrenheit reported no value for a sentence carrying a temperature")
	}
	if f < 55.9 || f > 56.0 {
		t.Errorf("Fahrenheit = %v, want about 55.9", f)
	}
}

// TestMTARejectsFahrenheitUnit covers the unit check. Unlike the fixed unit
// letters in the depth sentences, a temperature in Fahrenheit here is a 32
// degree error, which is the difference between a pleasant deck and a freezing
// one.
func TestMTARejectsFahrenheitUnit(t *testing.T) {
	if err := publishedErr(t, "IIMTA,55.9,F"); err == nil {
		t.Error("decoding an MTA in Fahrenheit succeeded, want an error")
	}
}

// TestMTABlankReading covers a probe with nothing to report, which is a normal
// state rather than a fault.
func TestMTABlankReading(t *testing.T) {
	m := published[MTA](t, "IIMTA,,C")
	if m.HasCelsius {
		t.Error("HasCelsius = true for a blank temperature")
	}
	if _, ok := m.Fahrenheit(); ok {
		t.Error("Fahrenheit reported a value for a blank temperature")
	}
	// A blank unit alongside a blank reading is an instrument with nothing to
	// say, not a mislabelled one.
	both := published[MTA](t, "IIMTA,,")
	if both.HasCelsius {
		t.Error("HasCelsius = true for an all-blank MTA")
	}
}

// TestMTAUpdatesTheFix checks that the air temperature reaches the fix, and is
// kept apart from the water temperature.
func TestMTAUpdatesTheFix(t *testing.T) {
	var f nmea.Fix
	published[MTA](t, "IIMTA,13.3,C").ApplyFix(&f)

	if !f.HasAirTemp || f.AirTempC != 13.3 {
		t.Errorf("fix air temperature = %v (present %v), want 13.3", f.AirTempC, f.HasAirTemp)
	}
	// The air temperature must not be filed as the water temperature: a
	// program asking for the sea temperature and handed the air temperature
	// would make a freezing-water decision on a mild day.
	if f.HasWaterTemp {
		t.Error("MTA set the water temperature, which it does not measure")
	}
}

// TestMTATruncatedRejected covers a sentence with no field at all.
func TestMTATruncatedRejected(t *testing.T) {
	if err := publishedErr(t, "IIMTA"); err == nil {
		t.Error("decoding an MTA with no fields succeeded, want an error")
	}
}
