package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// VER is the Version and Model Identification sentence. It is the only way a
// receiver describes itself: manufacturer, product, and software.
//
//	$GPVER,SiRF,GSiRF03,1000,1.00*5C
//	$GPRVER,SiRF03,3.2,1000*3E
//	$PMTKVER,3.00,0003*43
//
// Field layout:
//
//	0 manufacturer
//	1 product
//	2 software version
//	3 receiver version, commonly a build or model number
//
// The type is declared in the root nmea package so that the parser's receiver
// description can consume it without the two packages depending on each
// other; the decoder lives here.
//
// There is a vendor form, $PUBX,00, which reuses the VER address to list the
// formatters a receiver supports rather than its product details. That
// sentence is decoded as UBX instead, because its contents are a list of
// strings with no product information in them.
type ver struct{}

func (ver) Formatter() string { return "VER" }

func (ver) Decode(s nmea.Sentence) (any, error) {
	out := nmea.VER{
		Manufacturer: text(s.Field(0)),
		Product:      text(s.Field(1)),
		Software:     text(s.Field(2)),
		Receiver:     text(s.Field(3)),
		Talker:       s.Talker,
	}
	// Only treat field 3 as a standard revision when it really looks like
	// one. Receivers put build numbers there far more often than versions,
	// and reporting a build number of "1.00" as NMEA version 1.00 would be
	// nonsense that then propagates into every version-dependent decision.
	if v, ok := nmea.ParseVersion(s.Field(3)); ok {
		out.ProtocolVersion, out.HasProtocolVersion = v, true
	}
	// Some receivers state their standard version in the software field
	// instead, as "NMEA 4.11" or a bare "4.11".
	if !out.HasProtocolVersion {
		if v, ok := versionFromSoftware(out.Software); ok {
			out.ProtocolVersion, out.HasProtocolVersion = v, true
		}
	}
	return out, nil
}

// versionFromSoftware looks for a standard revision inside a free-text
// software string. It requires an explicit marker such as "NMEA" so that a
// product version of "4.11" is not mistaken for a standard revision by
// accident, and it also accepts the last dotted pair as a fallback for
// receivers that put the revision there with no marker at all.
func versionFromSoftware(s string) (nmea.Version, bool) {
	const marker = "NMEA"
	upper := s
	for i := 0; i+len(marker) <= len(upper); i++ {
		if equalFold(upper[i:i+len(marker)], marker) {
			rest := s[i+len(marker):]
			// Skip the "NMEA" label, any spaces, and an optional "v".
			j := 0
			for j < len(rest) && (rest[j] == ' ' || rest[j] == ':' || rest[j] == '=') {
				j++
			}
			if j < len(rest) && (rest[j] == 'v' || rest[j] == 'V') {
				j++
			}
			if v, ok := nmea.ParseVersion(rest[j:]); ok {
				return v, true
			}
		}
	}
	return nmea.Version{}, false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'a' <= ca && ca <= 'z' {
			ca -= 'a' - 'A'
		}
		if 'a' <= cb && cb <= 'z' {
			cb -= 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
