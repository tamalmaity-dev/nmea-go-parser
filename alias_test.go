package nmea

import (
	"errors"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"github.com/tamalmaity-dev/nmea-go-parser/value"
	"github.com/tamalmaity-dev/nmea-go-parser/wire"
)

// The re-exports are aliases, not conversions, so a decoder written against
// either spelling produces the same type. That is what lets the value types
// move out of this package without every decoder being rewritten.
func TestValueAliasesAreIdentical(t *testing.T) {
	// A value.TOD assigned to a nmea.TOD and back must be the same value,
	// which it would not be if either side were a distinct named type.
	var rt TOD = value.TOD{Hour: 12, Minute: 34, Second: 56, Fraction: 0.7, Available: true, Valid: true}
	var vt value.TOD = rt

	if rt != vt {
		t.Errorf("round trip changed the value: %+v vs %+v", rt, vt)
	}
	if (rt != value.TOD{}) && rt.Second != vt.Second {
		t.Error("fields diverged")
	}
}

// Errors must be testable through either package path, because a caller may
// hold a sentinel from one and compare against the other.
func TestFaultAliasesAreIdentical(t *testing.T) {
	if ErrBadChecksum != fault.ErrBadChecksum {
		t.Error("nmea.ErrBadChecksum and fault.ErrBadChecksum are different values")
	}
	if ErrNotValid != fault.ErrNotValid {
		t.Error("nmea.ErrNotValid and fault.ErrNotValid are different values")
	}
	// And a value produced by value.IsBlank must satisfy errors.Is from either.
	_, err := ParseFieldFloat("nope")
	if err == nil {
		t.Fatal("ParseFieldFloat(\"nope\") = nil error, want one")
	}
	if !errors.Is(err, fault.ErrFieldValue) {
		t.Errorf("error %v does not wrap fault.ErrFieldValue", err)
	}
	if !errors.Is(err, ErrFieldValue) {
		t.Errorf("error %v does not wrap nmea.ErrFieldValue", err)
	}
}

// The wire types are re-exported the same way, so a decoder that takes a
// nmea.Sentence and one that takes a wire.Sentence share a single method set.
func TestWireAliasesAreIdentical(t *testing.T) {
	var rs Sentence = wire.Sentence{Type: TypeRMC, Talker: "GP", Fields: []string{"a", "b"}}
	var ws wire.Sentence = rs

	if ws.Type != rs.Type || ws.Talker != rs.Talker || len(ws.Fields) != len(rs.Fields) {
		t.Errorf("round trip changed the sentence: %+v vs %+v", rs, ws)
	}
	if rs.Address() != "GPRMC" {
		t.Errorf("Address() = %q, want GPRMC", rs.Address())
	}
	// The method set must be shared, not merely the fields: a value satisfying
	// wire.DecodedSentence has to satisfy the aliased interface too.
	var _ wire.DecodedSentence = testDecoded{}
	var _ DecodedSentence = testDecoded{}

	// And framing through either package produces the same bytes.
	if Frame("GPGGA,123519,4807.038,N") != wire.Frame("GPGGA,123519,4807.038,N") {
		t.Error("Frame and wire.Frame disagree")
	}
}

type testDecoded struct{}

func (testDecoded) DataType() Type                     { return TypeRMC }
func (testDecoded) Talker() string                     { return "GP" }
func (testDecoded) Address() string                    { return "GPRMC" }
func (testDecoded) Raw() string                        { return "$GPRMC*00" }
func (testDecoded) Constellation() value.Constellation { return value.ConstellationGPS }
func (testDecoded) String() string                     { return "$GPRMC*00" }
