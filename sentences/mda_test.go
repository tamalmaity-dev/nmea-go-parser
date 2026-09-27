package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// MDA decoder tests.

// TestMDAPublishedLayout checks every one of the twenty fields, because a
// twenty-field sentence is where an off-by-one hides. The values are built from
// the documented field list rather than copied from a receiver.
func TestMDAPublishedLayout(t *testing.T) {
	m := published[MDA](t,
		"IIMDA,29.9,I,1.013,B,17.0,C,12.5,C,60.0,8.0,10.0,C,120.0,T,125.0,M,10.0,N,5.1,M")

	// Fields 0 to 3: the same pressure in two units.
	if !m.HasPressureInHg || m.PressureInHg != 29.9 {
		t.Errorf("field 0 inches of mercury = %v (present %v), want 29.9", m.PressureInHg, m.HasPressureInHg)
	}
	if !m.HasPressureBar || m.PressureBar != 1.013 {
		t.Errorf("field 2 bars = %v (present %v), want 1.013", m.PressureBar, m.HasPressureBar)
	}
	// Fields 4 to 7: the two temperatures.
	if !m.HasAirTemp || m.AirTempC != 17.0 {
		t.Errorf("field 4 air temperature = %v (present %v), want 17.0", m.AirTempC, m.HasAirTemp)
	}
	if !m.HasWaterTemp || m.WaterTempC != 12.5 {
		t.Errorf("field 6 water temperature = %v (present %v), want 12.5", m.WaterTempC, m.HasWaterTemp)
	}
	// Fields 8 and 9: the two humidities, both percentages.
	if !m.HasRelativeHumidity || m.RelativeHumidity != 60.0 {
		t.Errorf("field 8 relative humidity = %v (present %v), want 60.0",
			m.RelativeHumidity, m.HasRelativeHumidity)
	}
	if !m.HasAbsoluteHumidity || m.AbsoluteHumidity != 8.0 {
		t.Errorf("field 9 absolute humidity = %v (present %v), want 8.0",
			m.AbsoluteHumidity, m.HasAbsoluteHumidity)
	}
	// Field 10: dew point.
	if !m.HasDewPoint || m.DewPointC != 10.0 {
		t.Errorf("field 10 dew point = %v (present %v), want 10.0", m.DewPointC, m.HasDewPoint)
	}
	// Fields 12 to 15: the wind direction against two norths.
	if !m.HasWindDirectionTrue || m.WindDirectionTrue != 120.0 {
		t.Errorf("field 12 true direction = %v (present %v), want 120.0",
			m.WindDirectionTrue, m.HasWindDirectionTrue)
	}
	if !m.HasWindDirectionMag || m.WindDirectionMagnetic != 125.0 {
		t.Errorf("field 14 magnetic direction = %v (present %v), want 125.0",
			m.WindDirectionMagnetic, m.HasWindDirectionMag)
	}
	// Fields 16 to 19: the wind speed in two units.
	if !m.HasWindSpeed || m.WindSpeedKnots != 10.0 {
		t.Errorf("field 16 knots = %v (present %v), want 10.0", m.WindSpeedKnots, m.HasWindSpeed)
	}
	// 10 knots is 5.144 m/s; the sent 5.1 is the receiver's rounding, and the
	// derived value is preferred so the two do not disagree in the last digit.
	if math.Abs(m.SpeedMps-5.14) > 0.01 {
		t.Errorf("SpeedMps = %v, want about 5.14", m.SpeedMps)
	}
	if m.DataType() != "MDA" {
		t.Errorf("DataType = %q, want MDA", m.DataType())
	}
}

// TestMDAWeatherStationBlankWaterTemperature covers the shape the standard
// documents for WeatherStation, one of the instruments MDA came from: the water
// temperature field is simply blank. A decoder that treated it as malformed
// would reject the whole sentence and lose the eleven other readings with it.
func TestMDAWeatherStationBlankWaterTemperature(t *testing.T) {
	m := published[MDA](t, "IIMDA,29.9,I,1.013,B,17.0,C,,C,60.0,,45.0,C,120.0,T,125.0,M,10.0,N,5.1,M")

	if m.HasWaterTemp {
		t.Error("HasWaterTemp = true for a blank water temperature")
	}
	// Everything else must still have arrived.
	if !m.HasAirTemp || m.AirTempC != 17.0 {
		t.Errorf("air temperature = %v, want 17.0", m.AirTempC)
	}
	if !m.HasDewPoint || m.DewPointC != 45.0 {
		t.Errorf("dew point = %v, want 45.0", m.DewPointC)
	}
	if !m.HasWindSpeed || m.WindSpeedKnots != 10.0 {
		t.Errorf("wind speed = %v, want 10.0", m.WindSpeedKnots)
	}
}

// TestMDAPressureConversion covers the unit conversion, since a barometer
// display wants hectopascals and neither field is in them.
func TestMDAPressureConversion(t *testing.T) {
	bars := published[MDA](t, "IIMDA,,I,1.013,B")
	if got, ok := bars.PressureHectopascals(); !ok || math.Abs(got-1013) > 1e-6 {
		t.Errorf("1.013 bar = %v hPa (present %v), want 1013, true", got, ok)
	}

	inHg := published[MDA](t, "IIMDA,29.9,I")
	got, ok := inHg.PressureHectopascals()
	if !ok {
		t.Fatal("PressureHectopascals reported nothing for an inches-of-mercury reading")
	}
	// 29.9 inHg is about 1012.5 hPa, which is standard atmosphere.
	if got < 1010 || got > 1015 {
		t.Errorf("29.9 inHg = %v hPa, want about 1012.5", got)
	}

	none := published[MDA](t, "IIMDA,,I")
	if _, ok := none.PressureHectopascals(); ok {
		t.Error("PressureHectopascals reported a value with no pressure in the sentence")
	}
}

// TestMDAIsNotAnObservation checks that a weather station cannot move the fix.
// It reports a position nowhere, so folding it in would be meaningless.
func TestMDAIsNotAnObservation(t *testing.T) {
	var v any = published[MDA](t, "IIMDA,29.9,I,1.013,B,17.0,C")
	if _, ok := v.(nmea.FixContributor); ok {
		t.Error("MDA implements FixContributor, but it carries no positional data")
	}
}

// TestMDAPartialSentence covers a station reporting only some of its sensors.
func TestMDAPartialSentence(t *testing.T) {
	m := published[MDA](t, "IIMDA,29.9,I")
	if !m.HasPressureInHg {
		t.Error("HasPressureInHg = false for a one-field MDA")
	}
	if m.HasAirTemp || m.HasWindSpeed || m.HasDewPoint {
		t.Error("a one-field MDA reported readings it does not contain")
	}
}

// TestMDATruncatedRejected covers a sentence with nothing in it.
func TestMDATruncatedRejected(t *testing.T) {
	if err := publishedErr(t, "IIMDA"); err == nil {
		t.Error("decoding an MDA with no fields succeeded, want an error")
	}
}
