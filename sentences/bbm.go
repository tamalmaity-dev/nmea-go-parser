package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// BBM is the AIS Broadcast Binary Message: a binary message every station in
// range can hear, with no addressee.
//
//	!AIBBM,26,2,1,3,8,177KQJ5000G?tO`K>RA1wUbN0TKH,0*2C
//
// Field layout:
//
//	0 total fragments, 1 to 9
//	1 fragment number, 1 to 9
//	2 message identifier, 0 to 9
//	3 channel, A or B
//	4 VDL message number, ITU-R M.1371
//	5 payload, six-bit ASCII armouring
//	6 fill bits, 0 to 5
//
// BBM is ABM without the addressee, so its payload sits one field earlier. That
// single difference is the whole reason it is a separate sentence rather than a
// flag on ABM, and getting the payload index wrong by one produces a message
// that decodes to bytes shifted by a whole field, which is plausible enough to
// be believed.
type BBM struct {
	Base
	aisFragment
	// Channel is A or B.
	Channel    string
	HasChannel bool
	// VDLMessageNumber is the ITU-R M.1371 message type.
	VDLMessageNumber    int
	HasVDLMessageNumber bool
}

type bbm struct{}

func (bbm) Formatter() string { return "BBM" }

func (bbm) Decode(s nmea.Sentence) (any, error) {
	body, err := decodeAISFragment(s, 5, 6)
	out := BBM{Base: newBase(s), aisFragment: body}
	if err != nil {
		return out, err
	}
	out.Channel, out.HasChannel = text(s.Field(3)), text(s.Field(3)) != ""
	if n, ok, err := optionalInt(s, 4); err != nil {
		return out, err
	} else if ok {
		out.VDLMessageNumber, out.HasVDLMessageNumber = n, true
	}
	return out, nil
}
