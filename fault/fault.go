// Package fault holds the sentinel errors returned by the nmea parser and its decoders.
//
// Every error in this package is a sentinel created with errors.New, and the
// values that return them wrap the sentinel with context rather than replacing
// it. A caller therefore always tests with errors.Is and never compares error
// values directly, which keeps working as sentinels are wrapped with
// additional detail:
//
//	if errors.Is(err, fault.ErrBadChecksum) {
//	    // a checksum mismatch, whatever else the message says
//	}
//
// The errors live in their own package so that a decoder, which already
// imports the core nmea package for its field accessors, can report a fault
// without the two depending on each other. Nothing in the module imports this
// package for any other purpose.
package fault

import "errors"

// MaxSentenceLength is the total sentence size the NMEA 0183 standard
// allows, counting the '$', the body, the '*', and the two checksum digits.
// Receivers that exceed it exist, hence DefaultMaxLineLength, but a sentence
// this size or smaller is always legal.
const MaxSentenceLength = 82

// Sentinel errors returned by the parser and the built-in decoders.
// Wrap-aware: always test with errors.Is, never with ==.
var (
	// ErrNotSentence is reported for input that is not an NMEA sentence at
	// all (blank line, binary noise, plain text chatter on the wire).
	// The read loop silently discards these; they are never delivered to
	// error handlers.
	ErrNotSentence = errors.New("nmea: not a sentence")

	// ErrEmptyField is reported when a field that the specification
	// requires is empty. Receivers legitimately blank optional fields, so
	// decoders treat ErrEmptyField as "value unavailable" rather than as a
	// hard parse failure.
	ErrEmptyField = errors.New("nmea: field is empty")

	// ErrBadChecksum means the trailing *hh checksum did not match the XOR
	// of the sentence body. The sentence is still returned to handlers (it
	// is usually readable) but Event.Err is set and the payload is not
	// trusted, so it does not update the Fix state.
	ErrBadChecksum = errors.New("nmea: checksum mismatch")

	// ErrMalformedChecksum means a '*' was present but was not followed by
	// two hexadecimal digits, so the framing itself is broken and nothing in
	// the sentence can be trusted.
	ErrMalformedChecksum = errors.New("nmea: malformed checksum")

	// ErrMissingChecksum means the sentence carried no *hh suffix. The
	// standard makes the checksum mandatory, but receivers omit it often
	// enough that it is not treated as a failure.
	ErrMissingChecksum = errors.New("nmea: missing checksum")

	// ErrShortSentence means the sentence was too short to even classify.
	ErrShortSentence = errors.New("nmea: sentence too short")

	// ErrUnknownSentence means no decoder is registered for the sentence
	// type. The raw sentence is still delivered to OnEvent subscribers.
	ErrUnknownSentence = errors.New("nmea: no decoder for sentence type")

	// ErrFieldCount means the sentence had fewer fields than the
	// specification requires.
	ErrFieldCount = errors.New("nmea: too few fields")

	// ErrFieldValue means a field was present but could not be converted to
	// the expected type.
	ErrFieldValue = errors.New("nmea: invalid field value")

	// ErrFieldRange means a numeric field parsed cleanly but fell outside
	// the range the specification allows.
	ErrFieldRange = errors.New("nmea: value out of range")

	// ErrNotValid is returned by accessors on values that were never
	// populated, e.g. Fix.SpeedKnots before any RMC sentence arrives.
	ErrNotValid = errors.New("nmea: value not available")
)
