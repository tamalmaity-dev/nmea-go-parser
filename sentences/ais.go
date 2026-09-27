package sentences

import (
	"fmt"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// AIS payload characters are ASCII armouring of six-bit values, which is how a
// binary message travels through a sentence that can only carry text.
//
// Each character carries six bits. Values 0 to 39 sit at ASCII 48 to 87, and
// values 40 to 63 sit at ASCII 96 to 119. The gap between them, ASCII 88 to 95,
// carries no value at all, so a character there means the payload is corrupt
// rather than unusual.
const (
	// Values below this sit in the first, contiguous window.
	aisArmourWindow = 40
	// The first window spans this range.
	aisArmourLow  = 48
	aisArmourHigh = 87
	// The second window starts 8 higher, to leave the gap.
	aisArmourLowHigh  = 96
	aisArmourHighHigh = 119
)

// decodeAISPayload converts six-bit ASCII armouring into the bytes it carries.
//
// fillBits is how many bits of the last character are padding rather than data,
// which is 0, 2, or 4 in practice. A message that is not a whole number of bytes
// is the normal case for AIS: the payloads are 6, 8, 10, and so on bits, not
// bytes, so the trailing partial byte is dropped rather than zero-padded. A
// decoder that padded it would report a fabricated byte at the end of every
// message.
func decodeAISPayload(field string, fillBits int) ([]byte, error) {
	if fillBits < 0 || fillBits >= 6 {
		return nil, fmt.Errorf("%w: AIS fill bits is %d, want 0 to 5",
			nmea.ErrFieldValue, fillBits)
	}

	bits := len(field)*6 - fillBits
	if bits < 0 {
		return nil, fmt.Errorf("%w: %d fill bits exceed the %d bits in an AIS payload of %d characters",
			nmea.ErrFieldValue, fillBits, len(field)*6, len(field))
	}

	out := make([]byte, bits/8)
	var acc, held int
	pos := 0
	for i := 0; i < len(field); i++ {
		c := field[i]
		// The two windows are disjoint and the gap between them is invalid, so
		// the check is against both rather than against one overall range. A
		// character in the gap would otherwise decode to a plausible value and
		// corrupt the message silently.
		var v int
		switch {
		case c >= aisArmourLow && c <= aisArmourHigh:
			v = int(c) - aisArmourLow
		case c >= aisArmourLowHigh && c <= aisArmourHighHigh:
			v = int(c) - aisArmourLowHigh + aisArmourWindow
		default:
			return nil, fmt.Errorf(
				"%w: AIS payload byte %d is %#02x, which is not six-bit armouring (valid %#02x to %#02x and %#02x to %#02x)",
				nmea.ErrFieldValue, i, c, aisArmourLow, aisArmourHigh, aisArmourLowHigh, aisArmourHighHigh)
		}
		acc = acc<<6 | v
		held += 6
		for held >= 8 {
			held -= 8
			out[pos] = byte(acc >> held)
			pos++
		}
		// Keep only the bits still held, so acc cannot grow without bound over a
		// long payload.
		acc &= (1 << held) - 1
	}
	return out, nil
}

// encodeAISPayload is the inverse of decodeAISPayload, which is what a program
// configuring a transponder needs in order to send.
//
// fillBits is derived from the payload length rather than passed in, because it
// is determined by the message: six bits of padding make the total a multiple of
// eight.
func encodeAISPayload(payload []byte) (field string, fillBits int) {
	bits := len(payload) * 8
	chars := (bits + 5) / 6
	fillBits = chars*6 - bits

	var b []byte
	acc, held := 0, 0
	for _, p := range payload {
		acc = acc<<8 | int(p)
		held += 8
		for held >= 6 {
			held -= 6
			v := (acc >> held) & 0x3F
			if v >= aisArmourWindow {
				v += 8
			}
			b = append(b, byte(v+aisArmourLow))
		}
		acc &= (1 << held) - 1
	}
	if held > 0 {
		v := acc << (6 - held)
		if v >= aisArmourWindow {
			v += 8
		}
		b = append(b, byte(v+aisArmourLow))
	}
	return string(b), fillBits
}

// aisFragment is the part of an AIS message envelope every AIS sentence shares:
// the fragment count and number, a sequence identifier so fragments of one
// message can be reassembled, and the payload.
//
// The sequence number is what makes reassembly possible, and it is the field a
// decoder most often drops: without it, two interleaved multi-fragment messages
// cannot be told apart.
type aisFragment struct {
	TotalFragments    int
	FragmentNumber    int
	HasTotalFragments bool
	HasFragmentNumber bool
	// SequenceID groups the fragments of one message. 0 to 9, and absent on a
	// single-fragment message.
	SequenceID    int
	HasSequenceID bool
	// Channel is A or B for AIS, and is absent on a broadcast message.
	Channel    string
	HasChannel bool
	// Payload is the decoded binary message.
	Payload []byte
	// FillBits is the padding in the last character.
	FillBits int
}

// decodeAISFragment reads the shared envelope, given the field index of each
// part. The AIS sentences differ in where the payload sits and whether there is
// a channel, not in how the envelope is read.
func decodeAISFragment(s nmea.Sentence, payloadField, fillField int) (aisFragment, error) {
	var out aisFragment
	// The total fragment count is what makes the other fields meaningful, so a
	// sentence without it cannot be interpreted.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}

	var err error
	if out.TotalFragments, out.HasTotalFragments, err = optionalInt(s, 0); err != nil {
		return out, err
	}
	if out.FragmentNumber, out.HasFragmentNumber, err = optionalInt(s, 1); err != nil {
		return out, err
	}
	if out.SequenceID, out.HasSequenceID, err = optionalInt(s, 2); err != nil {
		return out, err
	}
	// A fragment number outside its cycle means the fragments cannot be
	// assembled, so it is worth catching rather than reporting a message that
	// will never complete.
	if out.HasFragmentNumber && out.HasTotalFragments &&
		(out.FragmentNumber < 1 || out.TotalFragments < 1 ||
			out.FragmentNumber > out.TotalFragments) {
		return out, fmt.Errorf(
			"%w: AIS fragment %d of %d is out of sequence",
			nmea.ErrFieldValue, out.FragmentNumber, out.TotalFragments)
	}
	// The fill-bit count is the one AIS field a receiver may leave blank, and a
	// missing count means no padding, so it goes through the optional reader
	// rather than being required.
	fill, _, err := optionalInt(s, fillField)
	if err != nil {
		return out, err
	}
	if out.Payload, err = decodeAISPayload(text(s.Field(payloadField)), fill); err != nil {
		return out, err
	}
	out.FillBits = fill
	return out, nil
}

// VDM and VDO share the same body, differing only in direction: VDM is a
// message received from another vessel, VDO is one this vessel sent. Both are
// decoded to the same type, with Direction saying which.
type VDMVDO struct {
	Base
	aisFragment
	// OwnShip says which direction the message travelled: false for VDM, a
	// message from another vessel, true for VDO, one this vessel sent.
	OwnShip bool
}

// Message returns the reassembly key and the payload, for a caller accumulating
// multi-fragment messages.
func (v VDMVDO) Message() (sequenceID int, payload []byte, ok bool) {
	// A single-fragment message is complete on its own. A multi-fragment one is
	// only complete at the last fragment, and even then the caller has to
	// concatenate, so this reports false for anything but the easy case.
	if !v.HasFragmentNumber || v.FragmentNumber != 1 {
		return 0, nil, false
	}
	if v.HasTotalFragments && v.TotalFragments > 1 {
		return 0, nil, false
	}
	return v.SequenceID, v.Payload, true
}

type vdm struct{}

func (vdm) Formatter() string { return "VDM" }

func (vdm) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeAISFragment(s, 4, 5)
	if err != nil {
		return VDMVDO{Base: newBase(s), aisFragment: body}, err
	}
	out := VDMVDO{Base: newBase(s), aisFragment: body}
	out.Channel, out.HasChannel = text(s.Field(3)), text(s.Field(3)) != ""
	return out, nil
}

type vdo struct{}

func (vdo) Formatter() string { return "VDO" }

func (vdo) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeAISFragment(s, 4, 5)
	if err != nil {
		return VDMVDO{Base: newBase(s), aisFragment: body}, err
	}
	out := VDMVDO{Base: newBase(s), aisFragment: body, OwnShip: true}
	out.Channel, out.HasChannel = text(s.Field(3)), text(s.Field(3)) != ""
	return out, nil
}

// ACK is the Alert Acknowledgement: a station confirming it has seen an alert.
//
//	$VRACK,001*50
//
// Field layout:
//
//	0 alert identifier, 001 to 99999
//
// ACK is a bare number, and that is all it is. The identifier refers to an alert
// sentence this library does not decode, so a program receiving one has an
// acknowledgement it cannot resolve to anything. It is decoded because it is one
// field and appears on alert-capable equipment, not because it is useful alone.
type ACK struct {
	Base
	// AlertIdentifier is the number from the alert being acknowledged.
	AlertIdentifier    int
	HasAlertIdentifier bool
}

type ack struct{}

func (ack) Formatter() string { return "ACK" }

func (ack) Decode(s nmea.Sentence) (any, error) {
	out := ACK{Base: newBase(s)}
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}
	var err error
	if out.AlertIdentifier, out.HasAlertIdentifier, err = optionalInt(s, 0); err != nil {
		return out, err
	}
	return out, nil
}
