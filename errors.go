package nmea

import "github.com/tamalmaity-dev/nmea-go-parser/fault"

// Sentinel errors, re-exported so existing callers keep working.
//
// The values live in the fault package, which holds them so that a decoder can
// report a fault without importing the core nmea package. They are aliased
// rather than re-declared so that errors.Is works identically against either
// path: nmea.ErrBadChecksum and fault.ErrBadChecksum are the same value.
var (
	ErrNotSentence       = fault.ErrNotSentence
	ErrEmptyField        = fault.ErrEmptyField
	ErrBadChecksum       = fault.ErrBadChecksum
	ErrMalformedChecksum = fault.ErrMalformedChecksum
	ErrMissingChecksum   = fault.ErrMissingChecksum
	ErrShortSentence     = fault.ErrShortSentence
	ErrUnknownSentence   = fault.ErrUnknownSentence
	ErrFieldCount        = fault.ErrFieldCount
	ErrFieldValue        = fault.ErrFieldValue
	ErrFieldRange        = fault.ErrFieldRange
	ErrNotValid          = fault.ErrNotValid
)
