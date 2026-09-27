// Package value holds the types a decoded sentence is made of: the values
// that arrive in a field, the enums the standard defines for them, and the
// formatting helpers that turn them back into text.
//
// # Why these are separate
//
// A value here knows nothing about sentences, framing, or the parser. That is
// what lets a decoder in the sentences package depend on the meaning of a
// field without depending on the machinery that delivered it, and it keeps
// the dependency graph a straight line rather than a knot:
//
//	fault   ← the sentinels
//	  ↑
//	value  ← this package: TOD, Date, Coordinate, Version, Constellation...
//	  ↑
//	nmea   ← framing, checksums, the registry, the parser, the aggregate Fix
//	  ↑
//	sentences, device
//
// # Blank is not zero
//
// Every optional value has a companion Valid, Available, or Has* flag,
// because a blank NMEA field means "not reported", which is not the same as
// zero. Latitude 0.0 and longitude 0.0 are real positions in the Gulf of
// Guinea, so no sentinel float is used anywhere in this package.
//
// # Two spellings
//
// These types are also reachable through the root nmea package, which aliases
// them. An alias is the same type rather than a new definition, so
// nmea.Coordinate and value.Coordinate are interchangeable and a value from
// one can be used wherever the other is expected. Import this package
// directly for a decoder or a library of your own; import the root package
// when you just want to parse a stream.
package value
