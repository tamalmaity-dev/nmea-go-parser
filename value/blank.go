package value

import "strings"

// IsBlank reports whether a raw NMEA field carries no value.
//
// A blank field is not the same as a zero one. NMEA spells an absent field
// either as nothing at all or as a single space, and receivers copy the
// standard's examples literally, so " " has to be treated as absent rather
// than parsed as a number.
//
// It lives here rather than beside the sentence type because every value
// parser in this package needs it, and so does the core package's field
// accessors.
func IsBlank(field string) bool { return strings.TrimSpace(field) == "" }
