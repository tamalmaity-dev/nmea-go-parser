package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// MDA is the Meteorological Composite sentence: a weather station's whole
// reading in one sentence.
//
//	$IIMDA,29.9,I,1.013,B,17.0,C,,C,60.0,,45.0,C,120.0,T,125.0,M,10.0,N,5.1,M
//
// Field layout, as value/unit pairs for the readings that carry a unit:
//
//	0  barometric pressure, inches of mercury   1  I
//	2  barometric pressure, bars                 3  B
//	4  air temperature, Celsius                  5  C
//	6  water temperature, Celsius                7  C
//	8  relative humidity, percent
//	9  absolute humidity, percent
//	10 dew point, Celsius                        11 C
//	12 wind direction, degrees true              13 T
//	14 wind direction, degrees magnetic          15 M
//	16 wind speed, knots                         17 N
//	18 wind speed, metres per second             19 M
//
// MDA is marked obsolete in the standard as of 2009, superseded by the
// individual sentences and by XDR. It is decoded because a great many weather
// instruments still send it, and because it is the only sentence that carries
// dew point and absolute humidity at all, neither of which has a modern
// replacement in common use.
//
// Two of the fields are worth singling out. Absolute humidity is documented as
// a percentage, which is unusual: it is a mass ratio rather than a relative
// measure, and the letter is the standard's own. The water temperature is
// documented as left blank by WeatherStation, one of the instruments this
// sentence came from, so a blank there is a known and expected shape rather
// than a fault.
type MDA struct {
	Base
	// PressureInHg and PressureBar are the same pressure in two units, as sent.
	PressureInHg    float64
	HasPressureInHg bool
	PressureBar     float64
	HasPressureBar  bool
	// AirTempC and WaterTempC are the two temperatures, in Celsius.
	AirTempC     float64
	HasAirTemp   bool
	WaterTempC   float64
	HasWaterTemp bool
	// RelativeHumidity and AbsoluteHumidity are both percentages.
	RelativeHumidity    float64
	HasRelativeHumidity bool
	AbsoluteHumidity    float64
	HasAbsoluteHumidity bool
	// DewPointC is the dew point, in Celsius.
	DewPointC   float64
	HasDewPoint bool
	// WindDirectionTrue and WindDirectionMagnetic are the same direction
	// against two norths.
	WindDirectionTrue     float64
	HasWindDirectionTrue  bool
	WindDirectionMagnetic float64
	HasWindDirectionMag   bool
	// WindSpeedKnots is the stored figure, with the other unit derived.
	WindSpeedKnots float64
	SpeedMps       float64
	HasWindSpeed   bool
}

type mda struct{}

func (mda) Formatter() string { return "MDA" }

func (mda) Decode(s nmea.Sentence) (any, error) {
	out := MDA{Base: newBase(s)}
	// The first pair is the shortest useful sentence, and the standard's own
	// example is the full twenty fields.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}

	var err error
	if out.PressureInHg, out.HasPressureInHg, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	if out.PressureBar, out.HasPressureBar, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.AirTempC, out.HasAirTemp, err = optionalFloat(s, 4); err != nil {
		return out, err
	}
	if out.WaterTempC, out.HasWaterTemp, err = optionalFloat(s, 6); err != nil {
		return out, err
	}
	if out.RelativeHumidity, out.HasRelativeHumidity, err = optionalFloat(s, 8); err != nil {
		return out, err
	}
	if out.AbsoluteHumidity, out.HasAbsoluteHumidity, err = optionalFloat(s, 9); err != nil {
		return out, err
	}
	if out.DewPointC, out.HasDewPoint, err = optionalFloat(s, 10); err != nil {
		return out, err
	}
	if out.WindDirectionTrue, out.HasWindDirectionTrue, err = optionalFloat(s, 12); err != nil {
		return out, err
	}
	if out.WindDirectionMagnetic, out.HasWindDirectionMag, err = optionalFloat(s, 14); err != nil {
		return out, err
	}
	if out.WindSpeedKnots, out.HasWindSpeed, err = optionalFloat(s, 16); err != nil {
		return out, err
	}
	if out.HasWindSpeed {
		// The m/s field is redundant with the knots figure and a receiver
		// rounds each independently, so the knots value is kept and the other
		// derived. Reading both would leave them disagreeing in the last digit
		// with no way to tell which was the measurement.
		out.SpeedMps = knotsToMps(out.WindSpeedKnots)
	}
	return out, nil
}

// PressureHectopascals returns the barometric pressure in hectopascals, the
// unit a barometer display shows, converting from whichever unit the receiver
// sent. 1 bar is 1000 hPa and 1 inch of mercury is about 33.8639 hPa.
func (m MDA) PressureHectopascals() (hpa float64, ok bool) {
	switch {
	case m.HasPressureBar:
		return m.PressureBar * 1000, true
	case m.HasPressureInHg:
		return m.PressureInHg * 33.8639, true
	default:
		return 0, false
	}
}
