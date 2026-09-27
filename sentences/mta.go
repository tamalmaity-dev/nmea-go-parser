package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// MTA is the Air Temperature sentence: a single air temperature in Celsius.
//
//	$IIMTA,13.3,C
//
// Field layout:
//
//	0 air temperature, degrees Celsius   1 C
//
// MTA is marked obsolete in the standard in favour of XDR, which carries a
// temperature alongside the vessel's other analogue inputs. It is decoded
// because weather instruments built long before that ruling still send it, and
// because it is three fields against XDR's structure, so a receiver with one
// probe and nothing else may emit only this.
//
// The unit is validated rather than ignored. Unlike the fixed unit letters in
// the depth sentences, a temperature in Fahrenheit here would be a 32 degree
// error, which is the difference between a pleasant deck and a freezing one, so
// the letter is worth insisting on.
type MTA struct {
	Base
	// Celsius is the air temperature in degrees Celsius.
	Celsius    float64
	HasCelsius bool
	// Unit is the letter as sent, which is C for every receiver this decoder
	// will accept.
	Unit string
}

type mta struct{}

func (mta) Formatter() string { return "MTA" }

func (mta) Decode(s nmea.Sentence) (any, error) {
	out := MTA{Base: newBase(s)}
	// A probe with no reading is a normal state, so the field may be blank.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}
	var err error
	if out.Celsius, out.HasCelsius, err = optionalFloat(s, 0); err != nil {
		return out, err
	}
	out.Unit = text(s.Field(1))
	// Only check the unit when there is a reading to mislabel. A blank
	// temperature with a blank unit is an instrument with nothing to say.
	if out.HasCelsius && out.Unit != "" && !referenceOK(out.Unit, "C") {
		return out, errMislabeled("MTA", "temperature", out.Unit)
	}
	return out, nil
}

// Fahrenheit converts the reading to degrees Fahrenheit.
func (m MTA) Fahrenheit() (f float64, ok bool) {
	if !m.HasCelsius {
		return 0, false
	}
	return m.Celsius*9/5 + 32, true
}

// ApplyFix folds the air temperature into the fix. It is kept apart from the
// water temperature, because a program that asked for the sea temperature and
// was handed the air temperature would make a freezing-water decision on a
// mild day.
func (m MTA) ApplyFix(f *nmea.Fix) {
	if m.HasCelsius {
		f.AirTempC, f.HasAirTemp = m.Celsius, true
	}
}
