package sentences

import (
	"bytes"
	"testing"
)

// TestPublishedBBM covers the broadcast envelope, whose payload sits one field
// earlier than ABM's because there is no addressee.
func TestPublishedBBM(t *testing.T) {
	b := published[BBM](t, "AIBBM,26,2,1,3,8,177KQJ5000G?tO`K>RA1wUbN0TKH,0")

	if !b.HasTotalFragments || b.TotalFragments != 26 {
		t.Errorf("total fragments = %v, want 26", b.TotalFragments)
	}
	if b.Channel != "3" || !b.HasChannel {
		t.Errorf("field 3 channel = %q (present %v), want 3", b.Channel, b.HasChannel)
	}
	if !b.HasVDLMessageNumber || b.VDLMessageNumber != 8 {
		t.Errorf("field 4 VDL message = %v, want 8", b.VDLMessageNumber)
	}
	// The payload must match the same armouring decoded for ABM and VDM.
	want, err := decodeAISPayload("177KQJ5000G?tO`K>RA1wUbN0TKH", 0)
	if err != nil {
		t.Fatalf("reference decode: %v", err)
	}
	if !bytes.Equal(b.Payload, want) {
		t.Errorf("payload = %x, want %x", b.Payload, want)
	}
}
