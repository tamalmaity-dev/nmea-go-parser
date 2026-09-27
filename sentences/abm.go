package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// ABM is the AIS Addressed Binary and Safety Related Message: a binary message
// aimed at one particular station, identified by its MMSI.
//
//	!AIABM,26,2,1,3381581370,3,8,177KQJ5000G?tO`K>RA1wUbN0TKH,0*02
//
// Field layout:
//
//	0 total fragments, 1 to 9
//	1 fragment number, 1 to 9
//	2 message identifier, 0 to 9
//	3 MMSI of the destination station, ten digits
//	4 channel, A or B
//	5 VDL message number, ITU-R M.1371
//	6 payload, six-bit ASCII armouring
//	7 fill bits, 0 to 5
//
// ABM is the outgoing counterpart of BBM: this vessel is sending to a named
// station rather than broadcasting. The MMSI is the only field that
// distinguishes the two, and it is what a reply has to quote.
//
// The payload stays as bytes. Decoding it into AIS message types is a different
// protocol from NMEA framing, and belongs to an AIS library rather than here.
type ABM struct {
	Base
	aisFragment
	// MMSI is the destination station, as sent. It is kept as a string because
	// it is an identity rather than a quantity, and because a ten-digit number
	// does not fit an int32 on every platform.
	MMSI    string
	HasMMSI bool
	// Channel is A or B.
	Channel    string
	HasChannel bool
	// VDLMessageNumber is the ITU-R M.1371 message type.
	VDLMessageNumber    int
	HasVDLMessageNumber bool
}

type abm struct{}

func (abm) Formatter() string { return "ABM" }

func (abm) Decode(s nmea.Sentence) (any, error) {
	// ABM's payload sits one field later than VDM's, because it carries the
	// destination MMSI first.
	body, err := decodeAISFragment(s, 6, 7)
	out := ABM{Base: newBase(s), aisFragment: body}
	if err != nil {
		return out, err
	}
	out.MMSI, out.HasMMSI = text(s.Field(3)), text(s.Field(3)) != ""
	out.Channel, out.HasChannel = text(s.Field(4)), text(s.Field(4)) != ""
	if n, ok, err := optionalInt(s, 5); err != nil {
		return out, err
	} else if ok {
		out.VDLMessageNumber, out.HasVDLMessageNumber = n, true
	}
	return out, nil
}
