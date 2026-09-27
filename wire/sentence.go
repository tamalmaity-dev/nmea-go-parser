package wire

import (
	"fmt"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"github.com/tamalmaity-dev/nmea-go-parser/value"
	"strings"
)

// Sentence is one decoded NMEA line, already split into its address and
// comma-separated fields but not yet interpreted.
//
// A Sentence is always safe to read, even when parsing failed: the raw
// text and the checksum verdict are preserved so handlers can log or
// forward garbage without losing the original.
type Sentence struct {
	// Raw is the line as received, minus the terminator and any leading
	// noise before the '$'.
	Raw string
	// Talker is the two-character source designator, "GP" for a generic
	// GPS receiver, "GN" for multi-constellation, "GL" for GLONASS-only,
	// and so on. It is empty for proprietary sentences, whose third and
	// fourth characters identify the manufacturer instead.
	Talker string
	// Type is the three-character sentence formatter, e.g. "GGA" or "RMC".
	Type string
	// Fields holds the comma-separated payload. Fields[0] is the first
	// field after the formatter, matching the numbering in the NMEA 0183
	// standard, so a GGA field index of 5 is the fix-quality field.
	Fields []string
	// Checksum is the value that arrived in the *hh suffix, and Computed is
	// the value the body actually implies. ChecksumState is the verdict;
	// comparing the two is how a caller reports the mismatch.
	Checksum      uint8
	Computed      uint8
	ChecksumState ChecksumStatus
	// Proprietary is true for $P sentences, which carry manufacturer data
	// rather than a standard formatter.
	Proprietary bool
	// TagBlock is the NMEA 4.10 metadata block that preceded this sentence, if
	// there was one. It carries its own checksum, verified separately, so a
	// sentence can be trusted while its tag block is not or the other way
	// round.
	TagBlock TagBlock
	// address is the talker and formatter, which are contiguous in the body,
	// so it is a slice of the already-parsed string rather than a
	// concatenation. Every decoded sentence records its own address in the
	// fix, and building it on demand allocated a string per sentence.
	address string
}

// ChecksumOK reports whether the sentence may be trusted, which means a
// valid checksum or none at all.
func (s Sentence) ChecksumOK() bool { return s.ChecksumState.OK() }

// Field returns the field at index i, or "" if i is out of range. Field 0
// is the first field after the sentence formatter.
func (s Sentence) Field(i int) string {
	if i < 0 || i >= len(s.Fields) {
		return ""
	}
	return s.Fields[i]
}

// HasFields reports whether the sentence carries at least n fields, so a
// caller can guard a run of indexed Field calls with one check.
func (s Sentence) HasFields(n int) bool { return len(s.Fields) >= n }

// Blank reports whether the field at index i is empty, out of range, or
// filled with nothing but spaces.
//
// The space case is not pedantry: the NMEA standard's own VTG example spells
// an absent field as a single space rather than an empty one, and receivers
// copy documents like that literally. Treating " " as a number is a parse
// failure on a perfectly good sentence.
func (s Sentence) Blank(i int) bool { return value.IsBlank(s.Field(i)) }

// Float parses the field at index i. A blank field yields fault.ErrEmptyField; an
// unparsable one yields fault.ErrFieldValue.
func (s Sentence) Float(i int) (float64, error) {
	raw := s.Field(i)
	if value.IsBlank(raw) {
		return 0, fault.ErrEmptyField
	}
	return value.ParseFieldFloat(raw)
}

// Int parses the field at index i as an integer. A blank field yields
// fault.ErrEmptyField; an unparsable one yields fault.ErrFieldValue.
func (s Sentence) Int(i int) (int, error) {
	raw := s.Field(i)
	if value.IsBlank(raw) {
		return 0, fault.ErrEmptyField
	}
	return value.ParseFieldInt(raw)
}

// Time parses the field at index i as an hhmmss[.sss] time of day.
func (s Sentence) Time(i int) (value.TOD, error) { return value.ParseTOD(s.Field(i)) }

// Date parses the field at index i as a ddmmyy date.
func (s Sentence) Date(i int) (value.Date, error) { return value.ParseDate(s.Field(i)) }

// Coordinate parses the field pair at indices i and i+1 as a
// latitude/longitude, range-checked against axis.
func (s Sentence) Coordinate(i int, axis value.Axis) (value.Coordinate, error) {
	return value.ParseCoordinate(s.Field(i), s.Field(i+1), axis)
}

// Quality parses the field at index i as a GGA fix-quality indicator.
func (s Sentence) Quality(i int) (value.FixQuality, error) { return value.ParseFixQuality(s.Field(i)) }

// Status parses the field at index i as an A/V navigation status.
func (s Sentence) Status(i int) (value.NavigationStatus, error) {
	return value.ParseNavigationStatus(s.Field(i))
}

// Tail returns everything from index i onward joined by commas, which is
// how variable-length sentences such as GSV groups and RTE waypoint lists
// are handled.
func (s Sentence) Tail(i int) []string {
	if i < 0 {
		i = 0
	}
	if i >= len(s.Fields) {
		return nil
	}
	return s.Fields[i:]
}

// Address returns the talker and formatter together, e.g. "GNGGA". For
// proprietary sentences it returns the Type alone.
//
// The value is taken from the sentence body rather than concatenated, so
// calling this on the hot path does not allocate.
func (s Sentence) Address() string {
	if s.address != "" {
		return s.address
	}
	return buildAddress(s.Talker, s.Type, s.Proprietary)
}

func buildAddress(talker, typ string, proprietary bool) string {
	if talker == "" {
		return typ
	}
	return talker + typ
}

func (s Sentence) String() string {
	return fmt.Sprintf("$%s,%s*%02X", s.Address(), strings.Join(s.Fields, ","), s.Computed)
}

// splitAddress divides the sentence body (no '$', no '*') into a talker, a
// three-character formatter, the address width, and whether it is
// proprietary.
//
// Standard sentences are TTSSS: a two-character talker followed by three
// characters of formatter, e.g. "GPGGA". Proprietary sentences are
// P<formatter><message id>, e.g. "PUBX" or "PMTK001", and have no talker.
//
// The width is measured rather than assumed, because a proprietary address
// has no fixed length: "PUBX" is four characters and "PMTK001" is seven.
// The only thing all of them share is that the address ends at the first
// comma, which is where the first field begins.
func splitAddress(body string) (talker, typ string, width int, proprietary bool) {
	if body == "" {
		return "", "", 0, false
	}

	width = len(body)
	if i := strings.IndexByte(body, ','); i >= 0 {
		width = i
	}
	address := body[:width]

	if address[0] == 'P' && len(address) >= 4 {
		// Confirm positions 1..3 are letters before trusting them as a
		// formatter; a malformed $P sentence falls through with the whole
		// address as the type, so its fields are still reachable.
		if isAlpha(address[1]) && isAlpha(address[2]) && isAlpha(address[3]) {
			return "", address[1:4], width, true
		}
		return "", address, width, true
	}
	if len(address) >= 5 && isAlpha(address[0]) && isAlpha(address[1]) && isAlpha(address[2]) {
		return address[0:2], address[2:5], width, false
	}
	return "", address, width, false
}

func isAlpha(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// ParseSentence splits one raw line into a Sentence. It does not interpret
// any field values, and it does not consult the decoder registry.
// Input may contain the '$' prefix, the '*hh' checksum suffix, and trailing
// NUL padding, all of which are optional. A leading BOM or partial garbage
// before the '$' is skipped, because serial links drop and duplicate bytes.
//
// A NMEA 4.10 tag block preceding the sentence is parsed and attached, and a
// '!' sentence, which is how encapsulated data such as AIS is framed, is
// accepted alongside '$'.
func ParseSentence(line string) (Sentence, error) {
	// The tag block comes first and is consumed before the sentence is framed,
	// because the two carry separate checksums and the sentence's own framing
	// starts after the block's closing backslash.
	tag, n, tagErr := ParseTagBlock(line)
	rest := line
	if n > 0 {
		rest = line[n:]
	}

	body, received, cstate := Split(rest)

	if body == "" {
		if tagErr != nil {
			return Sentence{Raw: line, TagBlock: tag}, tagErr
		}
		trimmed := TrimDecorations(rest)
		if strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, "!") {
			return Sentence{Raw: line, TagBlock: tag}, fault.ErrShortSentence
		}
		return Sentence{Raw: line, TagBlock: tag}, fault.ErrNotSentence
	}
	if tagErr != nil {
		return Sentence{Raw: line, TagBlock: tag}, tagErr
	}

	talker, typ, width, proprietary := splitAddress(body)
	if typ == "" {
		return Sentence{Raw: line, TagBlock: tag}, fault.ErrShortSentence
	}

	// Fields begin after the address and the comma that separates it from
	// the first field. A sentence with no comma has no fields at all, which
	// is different from one whose only field is blank, and decoders rely on
	// the difference to tell a truncated sentence from an empty one.
	var fields []string
	if after := body[width:]; strings.HasPrefix(after, ",") {
		fields = strings.Split(after[1:], ",")
	} else if after != "" {
		fields = []string{after}
	}

	// The talker and the formatter are adjacent in the body, so the address
	// is a slice of it rather than a new string. Only a proprietary sentence,
	// whose talker is empty, needs the formatter on its own.
	address := typ
	if talker != "" {
		address = body[:width]
	}

	return Sentence{
		Raw:           rawSentence(line),
		Talker:        talker,
		Type:          typ,
		Fields:        fields,
		Checksum:      received,
		Computed:      Checksum(body),
		ChecksumState: cstate,
		Proprietary:   proprietary,
		TagBlock:      tag,
		address:       address,
	}, nil
}

// rawSentence returns the sentence text starting at its '$', with the
// trailing terminator, NUL padding, and any byte order mark removed, so
// handlers never see the leading garbage or the transport padding.
func rawSentence(line string) string {
	s := TrimDecorations(line)
	if i := strings.IndexByte(s, '$'); i > 0 {
		return s[i:]
	}
	return s
}
