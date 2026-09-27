package wire

import (
	"fmt"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"strings"
)

// TagBlockSeparator is the backslash that opens and closes a tag block.
const TagBlockSeparator = '\\'

// TagBlock is the NMEA 4.10 metadata block that may precede a sentence.
//
//	\g:1-2-73874,n:157036,s:r003669945,c:1241544035*4A\!AIVDM,1,1,,A,...,0*13
//
// A tag block is a comma-separated list of key:value pairs wrapped in
// backslashes and carrying its own checksum, separate from the sentence's. The
// keys are:
//
//	c  int    UNIX time in seconds or milliseconds
//	d  string destination, at most 15 characters
//	g  string sentence grouping, such as 1-2-73874
//	n  int    line count
//	r  int    relative time in seconds
//	s  string source or station, at most 15 characters
//	t  string free text, at most 15 characters
//
// There is a trap worth stating plainly. IEC 62320-1, introduced with NMEA
// 4.00, uses the same block format with a partly different key set: grouping is
// xGy there rather than g, and the line count is x rather than n. A receiver
// implementing the IEC variant rather than the NMEA one will send keys this
// struct does not name, so an unrecognised key is kept in Extra rather than
// dropped. Silently discarding it would lose the metadata for exactly the
// receivers that sent it.
type TagBlock struct {
	// Time is the emission time as a Unix timestamp. The unit is not stated by
	// the standard, so it is reported as sent and TimeIsSeconds disambiguates.
	Time        int64
	HasTime     bool
	TimeIsMilli bool
	// RelativeTime is seconds relative to a group, which is how a receiver
	// timestamps a burst it sends faster than it can send whole seconds.
	RelativeTime    int64
	HasRelativeTime bool
	// Destination is the station the message is addressed to.
	Destination    string
	HasDestination bool
	// Grouping ties a set of sentences to one another.
	Grouping    string
	HasGrouping bool
	// LineCount is how many lines the grouped set spans.
	LineCount    int64
	HasLineCount bool
	// Source identifies the sending station.
	Source    string
	HasSource bool
	// Text is free text from the sender.
	Text    string
	HasText bool

	// Extra holds any key this library does not name, in the order received, so
	// a receiver using the IEC key set is not silently truncated.
	Extra []TagBlockField

	// Present is true when a tag block was actually present and parsed. A zero
	// TagBlock is otherwise indistinguishable from a block with no fields.
	Present bool
	// Raw is the tag block exactly as it arrived, backslashes and checksum
	// included.
	Raw string
	// Checksum is the tag block's own checksum, which is separate from the
	// sentence's and is verified separately.
	Checksum      uint8
	ChecksumState ChecksumStatus
}

// TagBlockField is one key and value the library does not have a named field
// for.
type TagBlockField struct {
	Key   string
	Value string
}

// Field returns the value for a key, whether or not this library names it, and
// whether it was present. It is the lookup for a receiver using the IEC 62320-1
// key set, where the same idea sits under a different letter.
func (t TagBlock) Field(key string) (string, bool) {
	switch key {
	case "c":
		if t.HasTime {
			return formatInt(t.Time), true
		}
	case "d":
		if t.HasDestination {
			return t.Destination, true
		}
	case "g":
		if t.HasGrouping {
			return t.Grouping, true
		}
	case "n":
		if t.HasLineCount {
			return formatInt(t.LineCount), true
		}
	case "r":
		if t.HasRelativeTime {
			return formatInt(t.RelativeTime), true
		}
	case "s":
		if t.HasSource {
			return t.Source, true
		}
	case "t":
		if t.HasText {
			return t.Text, true
		}
	}
	for _, e := range t.Extra {
		if e.Key == key {
			return e.Value, true
		}
	}
	return "", false
}

func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b []byte
	for v > 0 {
		b = append([]byte{byte('0' + v%10)}, b...)
		v /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// ParseTagBlock reads a leading tag block and reports how many bytes of the line
// it consumed, which is 0 when there is none.
//
// A tag block is only recognised when the line genuinely starts with a
// backslash. Looking for one anywhere in the line would misfire on a proprietary
// payload, which can contain any byte at all.
func ParseTagBlock(line string) (TagBlock, int, error) {
	var out TagBlock

	s := TrimDecorations(line)
	if !strings.HasPrefix(s, string(TagBlockSeparator)) {
		return out, 0, nil
	}

	// The block ends at the closing backslash, which is what distinguishes it
	// from the sentence that follows. If there is none, the line is truncated.
	end := strings.IndexByte(s[1:], TagBlockSeparator)
	if end < 0 {
		return out, 0, fmt.Errorf("%w: tag block has no closing %q",
			fault.ErrShortSentence, string(TagBlockSeparator))
	}
	// end is relative to s[1:], so the block itself is s[:end+1].
	block := s[1 : end+1]

	body, received, status := splitTagBlock(block)
	out.Present = true
	out.Raw = s[:end+2]
	out.Checksum = received
	out.ChecksumState = status

	for _, field := range strings.Split(body, ",") {
		if field == "" {
			// A trailing comma produces an empty field, which is not a key and
			// is skipped rather than treated as malformed.
			continue
		}
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			// No colon means a key with no value, which the format allows.
			key, value = field, ""
		}
		if err := out.set(key, value); err != nil {
			return out, end + 2, err
		}
	}
	return out, end + 2, nil
}

// splitTagBlock separates a tag block's body from its own checksum. The block
// carries one of its own, so it cannot go through Split, which expects a dollar
// sign and returns a sentence body.
func splitTagBlock(block string) (body string, received uint8, status ChecksumStatus) {
	i := strings.LastIndexByte(block, '*')
	if i < 0 {
		return block, 0, ChecksumAbsent
	}
	digits := strings.TrimRight(block[i+1:], "\x00 \t")
	body = block[:i]
	if len(digits) < 2 {
		return body, 0, ChecksumMalformed
	}
	v, err := parseHexByte(digits[:2])
	if err != nil {
		return body, 0, ChecksumMalformed
	}
	if received = v; received == Checksum(body) {
		return body, received, ChecksumValid
	}
	return body, received, ChecksumInvalid
}

// set records one key and value, rejecting a numeric key that is not a number.
func (t *TagBlock) set(key, value string) error {
	switch key {
	case "c":
		v, err := parseTagInt(value)
		if err != nil {
			return fmt.Errorf("%w: tag block c field is %q, want a Unix timestamp",
				fault.ErrFieldValue, value)
		}
		t.Time, t.HasTime = v, true
		// The standard says seconds or milliseconds and does not say which.
		// Anything past a few decades in seconds is far in the future, so a
		// large value is milliseconds. This is a judgement, and it is recorded
		// as TimeIsMilli so a caller can see it was inferred.
		t.TimeIsMilli = v > 1e11
	case "r":
		v, err := parseTagInt(value)
		if err != nil {
			return fmt.Errorf("%w: tag block r field is %q, want a number of seconds",
				fault.ErrFieldValue, value)
		}
		t.RelativeTime, t.HasRelativeTime = v, true
	case "n":
		v, err := parseTagInt(value)
		if err != nil {
			return fmt.Errorf("%w: tag block n field is %q, want a line count",
				fault.ErrFieldValue, value)
		}
		t.LineCount, t.HasLineCount = v, true
	case "d":
		t.Destination, t.HasDestination = value, true
	case "g":
		t.Grouping, t.HasGrouping = value, true
	case "s":
		t.Source, t.HasSource = value, true
	case "t":
		t.Text, t.HasText = value, true
	default:
		// An unrecognised key is kept rather than dropped, because the IEC
		// 62320-1 variant uses the same block with different letters and a
		// receiver using it would otherwise lose its metadata silently.
		t.Extra = append(t.Extra, TagBlockField{Key: key, Value: value})
	}
	return nil
}

// parseTagInt reads a decimal integer, tolerating surrounding space.
func parseTagInt(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fault.ErrEmptyField
	}
	var v int64
	neg := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 && (c == '-' || c == '+') {
			neg = c == '-'
			continue
		}
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%q is not a number", s)
		}
		v = v*10 + int64(c-'0')
	}
	if neg {
		v = -v
	}
	return v, nil
}
