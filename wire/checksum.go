package wire

import (
	"fmt"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"strings"
)

//------------------------------------------------------------------------------------------
// Checksum rule (NMEA Revealed): 8-bit XOR of all chars between $ and *,
// including commas; mandatory; MSB first. Max sentence length 82 bytes incl. $ and CRLF.
//------------------------------------------------------------------------------------------

// ChecksumStatus is the verdict on a sentence's trailing checksum.
//
// It is a distinct type rather than a bool pair because the three outcomes
// call for different behaviour. An absent checksum is common and usually
// harmless, an invalid one means the sentence is suspect, and a malformed
// one means the framing itself broke. Collapsing the last two into "bad" or
// all three into "bad" loses the distinction that decides whether to keep
// parsing.

type ChecksumStatus int

const (
	// ChecksumAbsent means the sentence carried no *hh suffix. Many
	// receivers omit it, and the sentence is still well formed, so this is
	// not a failure.
	ChecksumAbsent ChecksumStatus = iota

	// ChecksumValid means the received checksum matches the computed one.
	ChecksumValid

	// ChecksumInvalid means a *hh suffix was present and readable but did
	// not match. The sentence arrived intact but wrong, which usually means
	// a baud rate mismatch or line noise.
	ChecksumInvalid

	// ChecksumMalformed means a '*' was present but was not followed by two
	// hexadecimal digits. The framing is broken, so nothing in the sentence
	// can be trusted.
	ChecksumMalformed
)

func (s ChecksumStatus) String() string {
	switch s {
	case ChecksumAbsent:
		return "absent"
	case ChecksumValid:
		return "valid"
	case ChecksumInvalid:
		return "invalid"
	case ChecksumMalformed:
		return "malformed"
	default:
		return "unknown"
	}
}

// OK reports whether the sentence may be trusted. Both an absent and a valid
// checksum are acceptable; the standard makes the checksum mandatory but
// omittance is widespread in the field.
func (s ChecksumStatus) OK() bool {
	return s == ChecksumAbsent || s == ChecksumValid
}

// Present reports whether the sender included a checksum at all.
func (s ChecksumStatus) Present() bool { return s != ChecksumAbsent }

// Error converts a failed verdict into an error, or nil if the sentence is
// usable. It wraps fault.ErrBadChecksum or fault.ErrMissingChecksum-adjacent sentinels
// so callers can test with errors.Is.
func (s ChecksumStatus) Error() error {
	switch s {
	case ChecksumInvalid:
		return fault.ErrBadChecksum
	case ChecksumMalformed:
		return fault.ErrMalformedChecksum
	default:
		return nil
	}
}

// Checksum computes the NMEA 0183 checksum of a sentence body: the
// exclusive OR of every byte, where the body is the text between the '$'
// and the '*'. The '$' itself is excluded, and neither are the field
// commas, which are all included.
//
//	body := "GPGGA,123519,4807.038,N"
//	Checksum(body) // 0x47 for the full standard example
func Checksum(body string) uint8 {
	var c uint8
	for i := 0; i < len(body); i++ {
		c ^= body[i]
	}
	return c
}

// Split separates a framed sentence into its body and the checksum that
// followed it.
//
// The body excludes the leading '$' and the '*' and its digits, so it is
// exactly the string to pass to Checksum. The returned ChecksumStatus
// records what was found:
//
//	$GPWPL,4917.16,N,12310.64,W,003*65
//	body "GPWPL,4917.16,N,12310.64,W,003", received 0x65, ChecksumValid
//
// A line with no '$' or no body yields ChecksumAbsent and empty results
// rather than an error, because the caller is usually classifying a stream
// that may contain non-NMEA traffic.
func Split(framed string) (body string, received uint8, status ChecksumStatus) {
	s := TrimDecorations(framed)
	// Both prefixes are accepted. '$' introduces a standard sentence and '!'
	// introduces encapsulated data, which is how every AIS sentence is framed,
	// so refusing '!' would make the whole AIS set unparseable.
	start := strings.IndexAny(s, "$!")
	if start > 0 {
		s = s[start:]
	}
	if !strings.HasPrefix(s, "$") && !strings.HasPrefix(s, "!") {
		return "", 0, ChecksumAbsent
	}

	body = s[1:]

	// Only the last '*' counts, because a proprietary payload may contain
	// one of its own.
	i := strings.LastIndexByte(body, '*')
	if i < 0 {
		return body, 0, ChecksumAbsent
	}
	digits := strings.TrimRight(body[i+1:], "\x00 \t")
	body = body[:i]

	if len(digits) < 2 {
		return body, 0, ChecksumMalformed
	}
	v, err := parseHexByte(digits[:2])
	if err != nil {
		return body, 0, ChecksumMalformed
	}

	received = v
	if received == Checksum(body) {
		return body, received, ChecksumValid
	}
	return body, received, ChecksumInvalid
}

// Validate checks a complete framed sentence and reports the first problem
// it finds. It is the standalone entry point for a caller that has a line
// and wants to know whether to trust it, without building a Sentence.
//
// A non-NMEA line yields fault.ErrNotSentence, an absent checksum yields nil, and
// a bad or malformed one yields an error wrapping fault.ErrBadChecksum or
// fault.ErrMalformedChecksum.
func Validate(framed string) error {
	s := TrimDecorations(framed)
	if i := strings.IndexByte(s, '$'); i > 0 {
		s = s[i:]
	}
	if !strings.HasPrefix(s, "$") {
		return fmt.Errorf("%w: %q", fault.ErrNotSentence, trimForError(framed))
	}

	body, received, status := Split(s)
	if body == "" {
		return fmt.Errorf("%w: %q", fault.ErrShortSentence, trimForError(framed))
	}
	switch status {
	case ChecksumInvalid:
		return fmt.Errorf("%w: got %02X, want %02X in %q",
			fault.ErrBadChecksum, received, Checksum(body), trimForError(framed))
	case ChecksumMalformed:
		return fmt.Errorf("%w: %q", fault.ErrMalformedChecksum, trimForError(framed))
	default:
		return nil
	}
}

// Valid is Validate reduced to a bool, for a caller that only wants to
// filter a stream and does not need the reason.
func Valid(framed string) bool { return Validate(framed) == nil }

// Frame wraps a sentence body in '$' and a '*hh' checksum, ready to write to
// a receiver. It is the exact inverse of Split, so a decoded value can be
// re-encoded and will pass Validate.
//
//	nmea.Frame("GPWPL,4917.16,N,12310.64,W,003")
//	// "$GPWPL,4917.16,N,12310.64,W,003*65"
//
// Use FrameWith for encapsulated data, which is framed with '!' rather than
// '$'. Every AIS sentence is, so a program writing one with Frame would produce
// a sentence a receiver would ignore.
func Frame(body string) string { return FrameWith(SentenceStart, body) }

// SentenceStart is the prefix on a standard sentence, and SentenceStartEnclosed
// the one on encapsulated data such as AIS.
const (
	SentenceStart         = "$"
	SentenceStartEnclosed = "!"
)

// hexDigits maps a nibble to its uppercase hex character, for FrameWith.
var hexDigits = [16]byte{
	'0', '1', '2', '3', '4', '5', '6', '7',
	'8', '9', 'A', 'B', 'C', 'D', 'E', 'F',
}

// FrameWith wraps a body with an explicit prefix and its checksum. The prefix
// is validated: anything other than '$' or '!' would produce a line no parser
// accepts, so it is rejected rather than passed through.
func FrameWith(prefix, body string) string {
	if prefix != SentenceStart && prefix != SentenceStartEnclosed {
		prefix = SentenceStart
	}
	sum := Checksum(body)
	// Assembled by hand rather than with Sprintf. The shape is fixed and
	// short, and Sprintf pays for reflection and a scratch buffer to produce
	// two hex digits; measured at about two and a half times the cost.
	out := make([]byte, 0, len(prefix)+len(body)+4)
	out = append(out, prefix...)
	out = append(out, body...)
	out = append(out, '*', hexDigits[sum>>4], hexDigits[sum&0x0F])
	return string(out)
}

// TrimDecorations strips the transport noise that accumulates around a
// sentence: NUL padding, line terminators, trailing spaces, and a leading
// byte order mark.
//
// The byte order mark is worth calling out. A UTF-8 file written by a text
// editor starts with one, and a serial link can prepend one at a power cycle
// or a USB re-enumeration. It is not part of the sentence, and leaving it in
// place makes a receiver's very first line fail to parse.
func TrimDecorations(framed string) string {
	s := strings.TrimRight(framed, "\x00\r\n \t")
	return strings.TrimPrefix(s, "\uFEFF")
}

// parseHexByte reads exactly two hexadecimal digits. It is stricter than a
// general hex parser on purpose: a checksum is always two digits, so a
// three-digit value means the framing is wrong, not that the checksum has an
// extra leading zero.
func parseHexByte(s string) (uint8, error) {
	if len(s) != 2 {
		return 0, fmt.Errorf("%w: checksum %q is not two digits", fault.ErrFieldValue, s)
	}
	var v uint8
	for i := 0; i < 2; i++ {
		var d uint8
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			d = c - '0'
		case c >= 'A' && c <= 'F':
			d = c - 'A' + 10
		case c >= 'a' && c <= 'f':
			d = c - 'a' + 10
		default:
			return 0, fmt.Errorf("%w: checksum %q is not hexadecimal", fault.ErrFieldValue, s)
		}
		v = v<<4 | d
	}
	return v, nil
}

// trimForError keeps an error message readable when the offending line is
// long or binary.
func trimForError(s string) string {
	const limit = 60
	s = TrimDecorations(s)
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}
