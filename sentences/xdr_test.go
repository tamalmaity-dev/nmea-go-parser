package sentences

import (
	"strings"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// XDR decoder tests. The TestPublished function below checks every field
// against the example sentence in the NMEA documentation; these tests cover
// the behaviour the published example does not reach.

func TestPublishedXDR(t *testing.T) {
	// Published example 1: six sensor readings across three types, three of
	// which carry a blank name.
	x := published[XDR](t, "HCXDR,A,171,D,PITCH,A,-37,D,ROLL,G,367,,MAGX,G,2420,,MAGY,G,-8984,,MAGZ")

	if len(x.Measurements) != 5 {
		t.Fatalf("got %d measurements, want 5: the payload is a repeating quadruplet", len(x.Measurements))
	}

	// Fields 0 to 3: angular displacement, 171 degrees, named PITCH.
	if m := x.Measurements[0]; m.Type != TransducerAngular || m.Value != 171 ||
		m.HasValue == false || m.Name != "PITCH" {
		t.Errorf("measurement 0 = %+v, want angular 171 named PITCH", m)
	}
	// Fields 4 to 7: a negative angle, which is a real reading and not an
	// error.
	if m := x.Measurements[1]; m.Type != TransducerAngular || m.Value != -37 || m.Name != "ROLL" {
		t.Errorf("measurement 1 = %+v, want angular -37 named ROLL", m)
	}
	// Fields 8 onwards: generic readings with a blank unit and a blank name,
	// which is the normal shape for a magnetometer.
	for i, want := range []struct {
		value float64
		name  string
	}{
		{367, "MAGX"}, {2420, "MAGY"}, {-8984, "MAGZ"},
	} {
		m := x.Measurements[2+i]
		if m.Type != TransducerGeneric {
			t.Errorf("measurement %d type = %q, want generic", 2+i, m.Type)
		}
		if !m.HasValue || m.Value != want.value {
			t.Errorf("measurement %d value = %v (present %v), want %v",
				2+i, m.Value, m.HasValue, want.value)
		}
		if m.Name != want.name {
			t.Errorf("measurement %d name = %q, want %q", 2+i, m.Name, want.name)
		}
		// A blank unit must stay blank rather than defaulting to something.
		if m.Unit != XDRUnitNone {
			t.Errorf("measurement %d unit = %q, want none", 2+i, m.Unit)
		}
	}
}

// TestPublishedXDRSingleSensor covers the other published example: one sensor
// and nothing else, which is the shape a temperature probe sends.
func TestPublishedXDRSingleSensor(t *testing.T) {
	x := published[XDR](t, "SDXDR,C,23.15,C,WTHI")

	if len(x.Measurements) != 1 {
		t.Fatalf("got %d measurements, want 1", len(x.Measurements))
	}
	m, ok := x.Measurement(TransducerTemperature)
	if !ok {
		t.Fatal("Measurement(temperature) found nothing")
	}
	if !m.HasValue || m.Value != 23.15 {
		t.Errorf("value = %v (present %v), want 23.15", m.Value, m.HasValue)
	}
	if m.Unit != XDRUnitCelsius {
		t.Errorf("unit = %q, want C", m.Unit)
	}
	if got := m.UnitMeaning(); got != "degC" {
		t.Errorf("UnitMeaning = %q, want degC", got)
	}
	if m.Name != "WTHI" {
		t.Errorf("name = %q, want WTHI", m.Name)
	}
}

// TestXDRPressureUnitsAreAmbiguous covers the letter the standard reuses. P is
// percent for a humidity sensor and pascals for a pressure sensor, so the unit
// letter alone cannot say which, and a decoder that picks one produces a
// reading that is wrong by a factor of 100.
func TestXDRPressureUnitsAreAmbiguous(t *testing.T) {
	pressure := published[XDR](t, "IIXDR,P,0.9,B,PTYPE")
	m, ok := pressure.Measurement(TransducerPressure)
	if !ok {
		t.Fatal("Measurement(pressure) found nothing")
	}
	if got := m.UnitMeaning(); got != "bar" {
		t.Errorf("pressure unit meaning = %q, want bar", got)
	}
	if m.Name != "PTYPE" {
		t.Errorf("name = %q, want PTYPE", m.Name)
	}

	humidity := published[XDR](t, "IIXDR,H,45.5,P,HUM")
	hm, ok := humidity.Measurement(TransducerHumidity)
	if !ok {
		t.Fatal("Measurement(humidity) found nothing")
	}
	if got := hm.UnitMeaning(); got != "%" {
		t.Errorf("humidity unit meaning = %q, want %%", got)
	}
}

// TestXDRUnknownTypeIsNotAnError is the property that makes this decoder usable
// on real hardware. The format is explicitly extensible and manufacturers do
// invent letters, so rejecting an unknown one would break the sentence
// entirely, taking the temperature reading down with it.
func TestXDRUnknownTypeIsNotAnError(t *testing.T) {
	x := published[XDR](t, "SDXDR,C,23.15,C,WTHI,Z,7,Q,MYSTERY")

	if len(x.Measurements) != 2 {
		t.Fatalf("got %d measurements, want 2: an unknown type must not stop the sentence", len(x.Measurements))
	}
	m := x.Measurements[1]
	// The letter is kept as sent rather than collapsed to a zero value, so the
	// only information the receiver gave about that sensor survives.
	if m.Type.Known() {
		t.Errorf("type = %q, want a letter this library does not name", m.Type)
	}
	if m.Type != TransducerType('Z') {
		t.Errorf("type = %q, want the letter Z preserved", m.Type)
	}
	if m.TypeLetter != "Z" {
		t.Errorf("TypeLetter = %q, want Z", m.TypeLetter)
	}
	if m.UnitLetter != "Q" {
		t.Errorf("UnitLetter = %q, want Q", m.UnitLetter)
	}
	if !m.HasValue || m.Value != 7 {
		t.Errorf("value = %v (present %v), want 7", m.Value, m.HasValue)
	}
	if m.Name != "MYSTERY" {
		t.Errorf("name = %q, want MYSTERY", m.Name)
	}
	// An unrecognised unit has no name, rather than a wrong one.
	if got := m.UnitMeaning(); got != "" {
		t.Errorf("UnitMeaning = %q, want empty for an unknown unit", got)
	}
	// The known reading is untouched.
	if first := x.Measurements[0]; !first.HasValue || first.Value != 23.15 {
		t.Errorf("the first reading was disturbed: %+v", first)
	}
}

// TestXDRRejectsPartialSensor covers a truncated sentence. A quadruplet that
// stops half way leaves the value and the unit in the wrong fields, so it is a
// decode failure rather than something to guess at.
func TestXDRRejectsPartialSensor(t *testing.T) {
	if err := publishedErr(t, "SDXDR,C,23.15,C,WTHI,D"); err == nil {
		t.Error("decoding an XDR with a partial sensor succeeded, want an error")
	}
	if err := publishedErr(t, "SDXDR,C,23.15"); err == nil {
		t.Error("decoding an XDR with two fields succeeded, want an error")
	}
}

// TestXDRLookups covers the accessors, which are what a program actually uses:
// a device with four tanks has four readings of one type differing only by
// name.
func TestXDRLookups(t *testing.T) {
	x := published[XDR](t, "IIXDR,G,0.5,,PORT_STBD,G,0.4,,PORT_PORT,G,0.6,,STBD_STBD,C,21.0,C,SEA")

	if all := x.MeasurementsOfType(TransducerGeneric); len(all) != 3 {
		t.Fatalf("MeasurementsOfType(generic) = %d, want 3", len(all))
	}
	starboard, ok := x.Named("STBD_STBD")
	if !ok {
		t.Fatal("Named(STBD_STBD) found nothing")
	}
	if !starboard.HasValue || starboard.Value != 0.6 {
		t.Errorf("STBD_STBD = %+v, want 0.6", starboard)
	}
	if _, ok := x.Named("NOPE"); ok {
		t.Error("Named(NOPE) found something")
	}
	if _, ok := x.Measurement(TransducerVolume); ok {
		t.Error("Measurement(volume) found something in a sentence with none")
	}
	// The whole sentence on one line, for a log.
	line := x.String()
	for _, want := range []string{"generic", "0.5", "PORT_STBD", "temperature", "21", "SEA"} {
		if !strings.Contains(line, want) {
			t.Errorf("String() = %q, want it to contain %q", line, want)
		}
	}
}

// TestXDRBlankValue covers a sensor with nothing to report. The value may be
// blank without the quadruplet being malformed, which is why only the value
// has a presence flag.
func TestXDRBlankValue(t *testing.T) {
	x := published[XDR](t, "SDXDR,C,,C,WTHI")
	m := x.Measurements[0]
	if m.HasValue {
		t.Error("HasValue = true for a blank value")
	}
	if got := m.String(); !strings.Contains(got, "no reading") {
		t.Errorf("String() = %q, want it to say there is no reading", got)
	}
	// The type and name still came through.
	if m.Type != TransducerTemperature || m.Name != "WTHI" {
		t.Errorf("type or name lost on a blank value: %+v", m)
	}
}

// TestXDRDoesNotTouchTheFix checks that a sensor sentence is not an
// observation. A tank level is data the receiver measured about the vessel,
// not about where it is, and folding it into the fix would be meaningless.
//
// The assertion is that XDR does not implement FixContributor at all, so the
// parser never considers folding it in. That is stronger than checking the
// fix is unchanged, because it holds for every possible sentence.
func TestXDRDoesNotTouchTheFix(t *testing.T) {
	var v any = published[XDR](t, "IIXDR,P,0.9,B,PTYPE")
	if _, ok := v.(nmea.FixContributor); ok {
		t.Error("XDR implements FixContributor, but it carries no positional data")
	}
}
