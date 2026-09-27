// Package wire turns a byte stream into framed sentences.
//
// It is the layer below the parser and above the value types: it finds where
// one sentence ends and the next begins, verifies its checksum, strips the
// NMEA 4.10 tag block, and splits the body into a talker, a formatter, and
// numbered fields. It interprets no field values.
//
// # Why it is separate
//
// Framing is the one part of a parser that has to be exactly right for every
// sentence, including the ones a decoder has never seen. Keeping it in its own
// package means it can be tested against the recorded corpus on its own, with
// no decoder and no registry in the way, and that a change to how a line is
// split cannot quietly alter what a field means.
//
// # The pieces
//
//   - Sentence is a framed line: the talker, the formatter, the fields, the
//     checksum verdict, and the tag block.
//   - ParseSentence turns one line into a Sentence.
//   - Checksum, Split, Frame, FrameWith, and Validate are the checksum
//     primitives, usable on their own.
//   - LineSplitter buffers across reads, because a serial read returns an
//     arbitrary slice of bytes: three sentences and half a fourth, or one
//     sentence cut mid-field.
//   - Type, the Type constants, and DecodedSentence describe the shape a
//     decoded value must have.
//
// # Nothing here knows a sentence type
//
// There is no GGA or RMC in this package. A formatter is just the three
// characters after the talker, and what it means is somebody else's problem.
package wire
