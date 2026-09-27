package sentences

import (
	"fmt"
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// XDR is the Transducer Measurement sentence: a variable-length list of sensor
// readings, four fields at a time.

//--------------------------------------------------------------------------------------
//	$HCXDR,A,171,D,PITCH,A,-37,D,ROLL,G,367,,MAGX*41
//	$SDXDR,C,23.15,C,WTHI*70
//	$IIXDR,P,0.9,B,PTYPE*5B
//--------------------------------------------------------------------------------------

// Field layout, repeated until the sentence ends:
//
//	0 transducer type  a single letter, see TransducerType
//	1 value            the reading, in the unit at field 2
//	2 unit             a single letter, see TransducerUnit
//	3 name             the sensor's identifier, often blank
//
// XDR is the odd one out among NMEA sentences: every other one has a fixed
// layout, and this one has a repeating quadruplet whose length the receiver
// chooses. That is also what makes it so useful, because a single XDR carries
// every analogue input a device has, and a program that only understands
// position can ignore the rest of the sentence rather than having to parse
// sentences it does not have a decoder for.
//
// An unknown transducer type or unit is not an error. The format is explicitly
// extensible and receivers invent their own, so a decoder that rejected them
// would fail on real hardware the moment a manufacturer added a sensor. Both
// are exposed as raw letters alongside the typed value, so a caller can tell
// "not a temperature sensor" from "a temperature sensor I have no name for".
type XDR struct {
	Base
	// Measurements are the readings, in the order the receiver sent them.
	Measurements []XDRMeasurement
}

// XDRMeasurement is one sensor reading.
type XDRMeasurement struct {
	// Type is what the sensor measures: temperature, depth, pressure, and so
	// on. It is a byte rather than a string so an unrecognised letter is still
	// representable.
	Type TransducerType
	// TypeLetter is the raw letter as it arrived, which is all there is to go
	// on for a type this library does not name.
	TypeLetter string

	// Value is the reading in Unit. It is a plain number rather than a typed
	// quantity because the unit varies per measurement: a pressure in
	// hectopascals and one in pascals are both a float and a letter.
	Value    float64
	HasValue bool

	// Unit is what Value is measured in.
	Unit       TransducerUnit
	UnitLetter string

	// Name identifies the sensor, such as "PTYPE" or "WTHI". Many receivers
	// leave it blank, which is why it carries no presence flag: a blank name is
	// normal, not missing data.
	Name string
}

// TransducerType is the letter that says what a sensor measures.
type TransducerType byte

// The transducer types the documentation names. The list is open: receivers
// send letters outside it, and XDRMeasurement.TypeLetter preserves those.
const (
	TransducerUnknown       TransducerType = 0
	TransducerAngular       TransducerType = 'A' // angular displacement, degrees
	TransducerAbsoluteHumid TransducerType = 'B' // absolute humidity, kg/m3
	TransducerTemperature   TransducerType = 'C' // temperature
	TransducerDepth         TransducerType = 'D' // depth
	TransducerFrequency     TransducerType = 'F' // frequency, hertz
	TransducerGeneric       TransducerType = 'G' // generic, unit-defined
	TransducerHumidity      TransducerType = 'H' // relative humidity, percent
	TransducerCurrent       TransducerType = 'I' // electrical current
	TransducerSalinity      TransducerType = 'L' // salinity, parts per thousand
	TransducerForce         TransducerType = 'N' // force, newtons
	TransducerPressure      TransducerType = 'P' // pressure
	TransducerFlow          TransducerType = 'R' // flow rate
	TransducerSwitch        TransducerType = 'S' // switch or valve state
	TransducerTachometer    TransducerType = 'T' // tachometer, revolutions per minute
	TransducerVoltage       TransducerType = 'U' // voltage
	TransducerVolume        TransducerType = 'V' // volume
)

func (t TransducerType) String() string {
	switch t {
	case TransducerAngular:
		return "angular"
	case TransducerAbsoluteHumid:
		return "absolute humidity"
	case TransducerTemperature:
		return "temperature"
	case TransducerDepth:
		return "depth"
	case TransducerFrequency:
		return "frequency"
	case TransducerGeneric:
		return "generic"
	case TransducerHumidity:
		return "humidity"
	case TransducerCurrent:
		return "current"
	case TransducerSalinity:
		return "salinity"
	case TransducerForce:
		return "force"
	case TransducerPressure:
		return "pressure"
	case TransducerFlow:
		return "flow"
	case TransducerSwitch:
		return "switch"
	case TransducerTachometer:
		return "tachometer"
	case TransducerVoltage:
		return "voltage"
	case TransducerVolume:
		return "volume"
	case TransducerUnknown:
		return "unknown"
	default:
		// A letter this library has no name for. Printing the letter is more
		// useful than printing "unknown", because a log line naming the letter
		// can be matched against the receiver's manual.
		return string(rune(t)) + "?"
	}
}

// Known reports whether the type is one this library names. It is false for a
// letter outside the documented set, which is a normal thing to receive rather
// than a fault.
func (t TransducerType) Known() bool {
	switch t {
	case TransducerAngular, TransducerAbsoluteHumid, TransducerTemperature,
		TransducerDepth, TransducerFrequency, TransducerGeneric,
		TransducerHumidity, TransducerCurrent, TransducerSalinity,
		TransducerForce, TransducerPressure, TransducerFlow,
		TransducerSwitch, TransducerTachometer, TransducerVoltage,
		TransducerVolume:
		return true
	default:
		return false
	}
}

// TransducerUnit is the letter that says what a reading is measured in.
//
// It is stored as the raw letter rather than as a decoded name because the
// standard reuses letters: B is bars or binary, K is kelvin or kg/m3, M is
// metres or cubic metres, and P is percent or pascals. Which one applies
// depends on the transducer type, so there is no single name to store.
// UnitMeaning resolves the pair.
type TransducerUnit byte

// The unit letters the documentation names.
const (
	XDRUnitNone         TransducerUnit = 0
	XDRUnitAmpere       TransducerUnit = 'A'
	XDRUnitBar          TransducerUnit = 'B'
	XDRUnitCelsius      TransducerUnit = 'C'
	XDRUnitDegrees      TransducerUnit = 'D'
	XDRUnitHertz        TransducerUnit = 'H'
	XDRUnitLitres       TransducerUnit = 'I'
	XDRUnitKelvin       TransducerUnit = 'K'
	XDRUnitMetre        TransducerUnit = 'M'
	XDRUnitNewton       TransducerUnit = 'N'
	XDRUnitPercent      TransducerUnit = 'P'
	XDRUnitRPM          TransducerUnit = 'R'
	XDRUnitPartsPerThou TransducerUnit = 'S'
	XDRUnitVolt         TransducerUnit = 'V'
)

func (u TransducerUnit) String() string {
	if u == XDRUnitNone {
		return ""
	}
	return string(rune(u))
}

// UnitMeaning names the unit, using the transducer type to pick between the
// letters the standard reuses. It is the only way to tell bars from binary or
// pascals from percent, and it returns an empty string for a letter it does
// not recognise rather than guessing.
func (m XDRMeasurement) UnitMeaning() string {
	switch m.Unit {
	case XDRUnitAmpere:
		return "A"
	case XDRUnitBar:
		if m.Type == TransducerFlow {
			return "binary"
		}
		return "bar"
	case XDRUnitCelsius:
		return "degC"
	case XDRUnitDegrees:
		return "deg"
	case XDRUnitHertz:
		return "Hz"
	case XDRUnitLitres:
		return "l/s"
	case XDRUnitKelvin:
		if m.Type == TransducerAbsoluteHumid {
			return "kg/m3"
		}
		return "K"
	case XDRUnitMetre:
		if m.Type == TransducerVolume {
			return "m3"
		}
		return "m"
	case XDRUnitNewton:
		return "N"
	case XDRUnitPercent:
		// Percent is used for humidity and for switch duty; pascals for
		// pressure. A pressure sensor is the only one that means pascals.
		if m.Type == TransducerPressure {
			return "Pa"
		}
		return "%"
	case XDRUnitRPM:
		return "rpm"
	case XDRUnitPartsPerThou:
		return "ppt"
	case XDRUnitVolt:
		return "V"
	default:
		return ""
	}
}

// Measurement returns the first reading of the given type, which is what a
// program wants when it is after one specific sensor. A receiver may send
// several, for instance a temperature and a pressure, so this is the first
// match rather than the only one.
func (x XDR) Measurement(t TransducerType) (XDRMeasurement, bool) {
	for _, m := range x.Measurements {
		if m.Type == t {
			return m, true
		}
	}
	return XDRMeasurement{}, false
}

// MeasurementsOfType returns every reading of one type, for a device that
// reports several of the same kind, such as four tank levels.
func (x XDR) MeasurementsOfType(t TransducerType) []XDRMeasurement {
	var out []XDRMeasurement
	for _, m := range x.Measurements {
		if m.Type == t {
			out = append(out, m)
		}
	}
	return out
}

// Named returns the reading whose sensor identifier matches, and false when the
// sentence carried no such sensor. It is the lookup a multi-tank installation
// needs, where the readings differ only by name.
func (x XDR) Named(name string) (XDRMeasurement, bool) {
	for _, m := range x.Measurements {
		if m.Name == name {
			return m, true
		}
	}
	return XDRMeasurement{}, false
}

// String renders every reading on one line, for a log.
func (x XDR) String() string {
	parts := make([]string, 0, len(x.Measurements))
	for _, m := range x.Measurements {
		parts = append(parts, m.String())
	}
	return strings.Join(parts, ", ")
}

// String renders one reading, such as "temperature 23.15 C (WTHI)".
func (m XDRMeasurement) String() string {
	var b strings.Builder
	b.WriteString(m.Type.String())
	if m.HasValue {
		fmt.Fprintf(&b, " %g", m.Value)
		if u := m.UnitMeaning(); u != "" {
			b.WriteString(" " + u)
		}
	} else {
		b.WriteString(" (no reading)")
	}
	if m.Name != "" {
		b.WriteString(" (" + m.Name + ")")
	}
	return b.String()
}

type xdr struct{}

func (xdr) Formatter() string { return "XDR" }

func (xdr) Decode(s nmea.Sentence) (any, error) {
	out := XDR{Base: newBase(s)}

	// The payload is a whole number of quadruplets. A remainder means the
	// receiver sent a partial sensor, which cannot be interpreted: the value
	// and the unit it belongs to would end up in the wrong place.
	if n := len(s.Fields) % 4; n != 0 {
		return out, fmt.Errorf(
			"sentences: XDR has %d fields, which is not a whole number of sensor quadruplets: %w",
			len(s.Fields), nmea.ErrFieldCount)
	}
	if len(s.Fields) == 0 {
		return out, needFields(s, 4)
	}

	out.Measurements = make([]XDRMeasurement, 0, len(s.Fields)/4)
	for i := 0; i < len(s.Fields); i += 4 {
		m := XDRMeasurement{
			TypeLetter: text(s.Field(i)),
			UnitLetter: text(s.Field(i + 2)),
			Name:       text(s.Field(i + 3)),
		}
		// An unrecognised letter is not an error; it becomes TransducerUnknown
		// with the raw letter kept, so the reading is still reported.
		if c := m.TypeLetter[0]; m.TypeLetter != "" {
			m.Type = TransducerType(c)
		}
		if m.UnitLetter != "" {
			m.Unit = TransducerUnit(m.UnitLetter[0])
		}
		// The value is the one field allowed to be blank: a switch reports
		// state through its value, but a sensor with nothing to say may leave
		// it empty rather than dropping the whole quadruplet.
		var err error
		if m.Value, m.HasValue, err = optionalFloat(s, i+1); err != nil {
			return out, err
		}
		out.Measurements = append(out.Measurements, m)
	}
	return out, nil
}
