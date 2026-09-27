package sentences

import (
	"bytes"
	"testing"
)

// TestPublishedABM verifies the addressed envelope, whose payload sits one
// field later than VDM's because of the destination MMSI.
func TestPublishedABM(t *testing.T) {
	a := published[ABM](t, "AIABM,26,2,1,3381581370,3,8,177KQJ5000G?tO`K>RA1wUbN0TKH,0")

	if !a.HasTotalFragments || a.TotalFragments != 26 {
		t.Errorf("field 0 total fragments = %v, want 26", a.TotalFragments)
	}
	if !a.HasFragmentNumber || a.FragmentNumber != 2 {
		t.Errorf("field 1 fragment number = %v, want 2", a.FragmentNumber)
	}
	if !a.HasSequenceID || a.SequenceID != 1 {
		t.Errorf("field 2 message id = %v (present %v), want 1", a.SequenceID, a.HasSequenceID)
	}
	// Field 3: the destination MMSI, kept as a string because it is an
	// identity and a ten-digit number overflows an int32.
	if a.MMSI != "3381581370" || !a.HasMMSI {
		t.Errorf("field 3 MMSI = %q (present %v), want 3381581370", a.MMSI, a.HasMMSI)
	}
	// Field 4: the channel.
	if a.Channel != "3" || !a.HasChannel {
		t.Errorf("field 4 channel = %q (present %v), want 3", a.Channel, a.HasChannel)
	}
	// Field 5: the VDL message number.
	if !a.HasVDLMessageNumber || a.VDLMessageNumber != 8 {
		t.Errorf("field 5 VDL message = %v (present %v), want 8", a.VDLMessageNumber, a.HasVDLMessageNumber)
	}
	// The payload must be the same bytes as the VDM carrying the same armouring,
	// which is what proves the payload index is right despite the extra fields
	// before it.
	want, err := decodeAISPayload("177KQJ5000G?tO`K>RA1wUbN0TKH", 0)
	if err != nil {
		t.Fatalf("reference decode: %v", err)
	}
	if !bytes.Equal(a.Payload, want) {
		t.Errorf("payload = %x, want %x", a.Payload, want)
	}
}
