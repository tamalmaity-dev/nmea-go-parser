package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// MTW is the Mean Temperature of Water sentence.
//
//	$INMTW,17.9,C
//	$SDMTW,15.5,C
//
// Field layout:
//
//	0 temperature degrees Celsius
//	1 unit         C for Celsius
//
// Despite the name, some documentation calls this the meteorological water
// temperature, which it is not: it is the temperature at the transducer, so it
// reflects the water the hull is sitting in rather than the air.
type MTW struct {
	Base
	// Celsius is the water temperature in degrees Celsius.
	Celsius    float64
	HasCelsius bool
	// Unit is C for Celsius. The only defined unit, but it is checked so a
	// receiver reporting Fahrenheit is caught rather than misread.
	Unit string
}

// Fahrenheit converts the reading to degrees Fahrenheit.
func (m *MTW) Fahrenheit() (f float64, ok bool) {
	if !m.HasCelsius {
		return 0, false
	}
	return m.Celsius*9/5 + 32, true
}

type mtw struct{}

func (mtw) Formatter() string { return "MTW" }

func (mtw) Decode(s nmea.Sentence) (any, error) {
	// A transducer that is not reporting leaves the field blank, so the
	// sentence may legitimately arrive with nothing in it.
	if len(s.Fields) == 0 {
		return nil, needFields(s, 1)
	}
	out := MTW{Base: newBase(s)}

	var err error
	if out.Celsius, out.HasCelsius, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	out.Unit = text(s.Field(1))
	if out.HasCelsius && out.Unit != "" && !referenceOK(out.Unit, "C") {
		return out, errMislabeled("MTW", "temperature", out.Unit)
	}
	return out, nil
}

// ApplyFix folds the MTW into the fix state. Water temperature is a
// measurement of the environment rather than of the position, so it is
// recorded as such.
func (m MTW) ApplyFix(f *nmea.Fix) {
	if m.HasCelsius {
		f.WaterTempC, f.HasWaterTemp = m.Celsius, true
	}
}
