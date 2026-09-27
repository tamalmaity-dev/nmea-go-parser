package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// AIS decoder tests. The payload stays as bytes: decoding it into AIS message
// types is a different protocol from NMEA framing and belongs to an AIS library,
// so what is checked here is the envelope and the six-bit armouring.

// TestAISPayloadArmouringRoundTrip is the core check on the armouring. The
// algorithm is the one from the gpsd AIVDM documentation, and an error in it
// produces bytes that are plausible and wrong, which is the worst kind of bug in
// a protocol decoder.
func TestAISPayloadArmouringRoundTrip(t *testing.T) {
	// The published example payload and its armouring. The string is 28
	// characters, and 28 times six bits is 168, which is the length of an AIS
	// position report: 21 bytes.
	const armour = "177KQJ5000G?tO`K>RA1wUbN0TKH"

	if len(armour) != 28 {
		t.Fatalf("the example payload is %d characters, want 28", len(armour))
	}
	payload, err := decodeAISPayload(armour, 0)
	if err != nil {
		t.Fatalf("decodeAISPayload: %v", err)
	}
	if len(payload) != 21 {
		t.Fatalf("got %d bytes, want 21: 28 characters of six bits is 168 bits", len(payload))
	}

	// Re-encoding must reproduce the exact characters, which is what a
	// transponder needs and what proves the packing order is the same in both
	// directions.
	back, fill := encodeAISPayload(payload)
	if fill != 0 {
		t.Errorf("re-encoded fill bits = %d, want 0", fill)
	}
	if back != armour {
		t.Errorf("re-encoded payload = %q, want %q", back, armour)
	}
}

// TestAISPayloadDropsPartialByte covers the padding rule. AIS payloads are 6, 8,
// 10 bits and so on, not whole bytes, so the trailing partial byte is dropped
// rather than zero-padded. Padding it would append a fabricated byte to every
// message.
func TestAISPayloadDropsPartialByte(t *testing.T) {
	// One character of six bits with two fill bits leaves four bits, which is no
	// whole byte.
	payload, err := decodeAISPayload("1", 2)
	if err != nil {
		t.Fatalf("decodeAISPayload: %v", err)
	}
	if len(payload) != 0 {
		t.Errorf("got %d bytes, want 0: four bits is not a byte", len(payload))
	}

	// Two fill bits on a three character payload leaves sixteen bits, which is
	// two whole bytes.
	payload, err = decodeAISPayload("177", 2)
	if err != nil {
		t.Fatalf("decodeAISPayload: %v", err)
	}
	if len(payload) != 2 {
		t.Errorf("got %d bytes, want 2: 18 bits minus 2 is 16", len(payload))
	}
}

// TestAISPayloadRejectsBadInput covers the two ways armouring can be wrong: a
// character outside the printable range, and a fill count that cannot exist.
func TestAISPayloadRejectsBadInput(t *testing.T) {
	// A character below 48 or above 119 is not six-bit armouring.
	for _, bad := range []string{"177KQJ5000G?tO`K>RA1wUbN0TK\x01", "~~~", " "} {
		if _, err := decodeAISPayload(bad, 0); err == nil {
			t.Errorf("decodeAISPayload(%q) succeeded, want an error", bad)
		}
	}
	// The gap between the two windows, ASCII 88 to 95, carries no value. A
	// character there must be rejected rather than decoded to something
	// plausible, because silently accepting it corrupts the message in a way
	// nothing downstream would notice.
	for c := 88; c <= 95; c++ {
		if _, err := decodeAISPayload("1"+string(rune(c)), 0); err == nil {
			t.Errorf("decodeAISPayload accepted %#02x, which is in the gap between the windows", c)
		}
	}
	// The window boundaries themselves are valid.
	for _, ok := range []int{48, 87, 96, 119} {
		if _, err := decodeAISPayload(string(rune(ok)), 0); err != nil {
			t.Errorf("decodeAISPayload rejected %#02x, which is a window boundary: %v", ok, err)
		}
	}
	// A fill count of six or more cannot leave any data at all.
	for _, fill := range []int{6, 7, -1} {
		if _, err := decodeAISPayload("177", fill); err == nil {
			t.Errorf("decodeAISPayload with %d fill bits succeeded, want an error", fill)
		}
	}
}

// TestPublishedVDM verifies the received-AIS envelope.
func TestPublishedVDM(t *testing.T) {
	v := published[VDMVDO](t, "AIVDM,1,1,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0")

	if !v.HasTotalFragments || v.TotalFragments != 1 {
		t.Errorf("field 0 total fragments = %v (present %v), want 1", v.TotalFragments, v.HasTotalFragments)
	}
	if !v.HasFragmentNumber || v.FragmentNumber != 1 {
		t.Errorf("field 1 fragment number = %v (present %v), want 1", v.FragmentNumber, v.HasFragmentNumber)
	}
	// Field 2: the sequence id, absent on a single-fragment message.
	if v.HasSequenceID {
		t.Error("HasSequenceID = true for a blank sequence field")
	}
	// Field 3: the channel.
	if v.Channel != "B" || !v.HasChannel {
		t.Errorf("field 3 channel = %q (present %v), want B", v.Channel, v.HasChannel)
	}
	if len(v.Payload) != 21 {
		t.Errorf("payload is %d bytes, want 21", len(v.Payload))
	}
	if v.OwnShip {
		t.Error("OwnShip = true for a VDM, which is a message from another vessel")
	}
	if v.DataType() != "VDM" {
		t.Errorf("DataType = %q, want VDM", v.DataType())
	}

	// A single-fragment message is complete on its own.
	if _, payload, ok := v.Message(); !ok || len(payload) != 21 {
		t.Errorf("Message reported %d bytes, %v; want 21, true", len(payload), ok)
	}
}

// TestPublishedVDOChecksDirection covers VDO, which shares VDM's body and
// differs only in direction.
func TestPublishedVDOChecksDirection(t *testing.T) {
	v := published[VDMVDO](t, "AIVDO,1,1,,A,177KQJ5000G?tO`K>RA1wUbN0TKH,0")
	if !v.OwnShip {
		t.Error("OwnShip = false for a VDO, which is a message this vessel sent")
	}
	if v.DataType() != "VDO" {
		t.Errorf("DataType = %q, want VDO", v.DataType())
	}
	if v.Channel != "A" {
		t.Errorf("channel = %q, want A", v.Channel)
	}
}

// TestAISSequenceIDGroupsFragments covers the field that makes multi-fragment
// messages reassemblable. Two messages interleaved on the same channel are told
// apart only by their sequence id.
func TestAISSequenceIDGroupsFragments(t *testing.T) {
	first := published[VDMVDO](t, "AIVDM,2,1,3,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0")
	second := published[VDMVDO](t, "AIVDM,2,2,3,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0")
	other := published[VDMVDO](t, "AIVDM,2,1,7,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0")

	if !first.HasSequenceID || first.SequenceID != 3 {
		t.Errorf("sequence id = %v (present %v), want 3", first.SequenceID, first.HasSequenceID)
	}
	if first.SequenceID != second.SequenceID {
		t.Errorf("the two fragments of one message have sequence ids %d and %d, want them equal",
			first.SequenceID, second.SequenceID)
	}
	if first.SequenceID == other.SequenceID {
		t.Error("two different messages share a sequence id, so they could not be told apart")
	}

	// A fragment of a multi-fragment message is not a message on its own.
	if _, _, ok := first.Message(); ok {
		t.Error("Message reported a single-fragment message for fragment 1 of 2")
	}
}

// TestAISRejectsOutOfSequenceFragment covers the check that a fragment number
// fits its cycle. Without it a receiver emitting nonsense produces a message
// that will never complete, silently.
func TestAISRejectsOutOfSequenceFragment(t *testing.T) {
	for _, body := range []string{
		"AIVDM,2,3,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0", // fragment 3 of 2
		"AIVDM,2,0,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0", // fragment 0 of 2
		"AIVDM,0,1,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0", // zero fragments
	} {
		if err := publishedErr(t, body); err == nil {
			t.Errorf("decoding %q succeeded, want an error", body)
		}
	}
}

// TestPublishedACK covers the alert acknowledgement, which is one field.
func TestPublishedACK(t *testing.T) {
	a := published[ACK](t, "VRACK,001")
	if !a.HasAlertIdentifier || a.AlertIdentifier != 1 {
		t.Errorf("field 0 alert identifier = %v (present %v), want 1",
			a.AlertIdentifier, a.HasAlertIdentifier)
	}
	if a.DataType() != "ACK" {
		t.Errorf("DataType = %q, want ACK", a.DataType())
	}
}

// TestACKTruncated covers an acknowledgement with no identifier, which says
// nothing about which alert was acknowledged.
func TestACKTruncated(t *testing.T) {
	if err := publishedErr(t, "VRACK"); err == nil {
		t.Error("decoding an ACK with no fields succeeded, want an error")
	}
}

// TestAISIsNotAnObservation checks that an AIS message cannot move the fix. AIS
// reports other vessels' positions, which say nothing about where this one is.
func TestAISIsNotAnObservation(t *testing.T) {
	for _, v := range []any{
		published[VDMVDO](t, "AIVDM,1,1,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0"),
		published[ABM](t, "AIABM,26,2,1,3381581370,3,8,177KQJ5000G?tO`K>RA1wUbN0TKH,0"),
		published[BBM](t, "AIBBM,26,2,1,3,8,177KQJ5000G?tO`K>RA1wUbN0TKH,0"),
		published[ACK](t, "VRACK,001"),
	} {
		if _, ok := v.(nmea.FixContributor); ok {
			t.Errorf("%T implements FixContributor, but it carries no vessel position", v)
		}
	}
}
