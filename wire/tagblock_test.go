package wire

import (
	"testing"
)

// The published example from the gpsd AIVDM documentation: a tag block
// preceding an AIS sentence. The block checksum is 4A and the sentence's is 13,
// both as printed in the source.
//
//	\g:1-2-73874,n:157036,s:r003669945,c:1241544035*4A\!AIVDM,1,1,,A,...,0*13
const taggedAIS = `\g:1-2-73874,n:157036,s:r003669945,c:1241544035*4A\!AIVDM,1,1,,B,15N4cJ` + "`" + `005Jrek0H@9n` + "`" + `DW5608EP,0*13`

// tick is a backtick, which appears in every AIS payload and so cannot be
// written inside a raw string literal.
const tick = "`"

// aisPayloadBody is a real AIS payload body, the one from the documentation.
const aisPayloadBody = "AIVDM,1,1,,A,177KQJ5000G?tO" + tick + "K>RA1wUbN0TKH,0"

// bang frames an encapsulated-data sentence the way a receiver does, with '!'
// and a correct checksum, so a test can vary the tag block without having to
// compute a checksum by hand.
func bang(body string) string {
	return "!" + body + "*" + hexByte(Checksum(body))
}

func hexByte(v uint8) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[v>>4], digits[v&0x0F]})
}

// TestParseTagBlock covers the documented example, checking every named field.
func TestParseTagBlock(t *testing.T) {
	tag, n, err := ParseTagBlock(taggedAIS)
	if err != nil {
		t.Fatalf("ParseTagBlock: %v", err)
	}
	if n == 0 {
		t.Fatal("ParseTagBlock consumed nothing for a line that starts with a tag block")
	}
	if !tag.Present {
		t.Error("Present = false for a line that carries a tag block")
	}

	// Field g: the sentence grouping.
	if !tag.HasGrouping || tag.Grouping != "1-2-73874" {
		t.Errorf("g grouping = %q (present %v), want 1-2-73874", tag.Grouping, tag.HasGrouping)
	}
	// Field n: the line count.
	if !tag.HasLineCount || tag.LineCount != 157036 {
		t.Errorf("n line count = %v (present %v), want 157036", tag.LineCount, tag.HasLineCount)
	}
	// Field s: the source.
	if !tag.HasSource || tag.Source != "r003669945" {
		t.Errorf("s source = %q (present %v), want r003669945", tag.Source, tag.HasSource)
	}
	// Field c: the emission time.
	if !tag.HasTime || tag.Time != 1241544035 {
		t.Errorf("c time = %v (present %v), want 1241544035", tag.Time, tag.HasTime)
	}
	if tag.TimeIsMilli {
		t.Error("TimeIsMilli = true for a seconds timestamp")
	}

	// The block's own checksum is separate from the sentence's.
	if tag.ChecksumState != ChecksumValid {
		t.Errorf("tag block checksum state = %v, want valid", tag.ChecksumState)
	}
	if tag.Raw == "" {
		t.Error("Raw was not recorded")
	}
	if len(tag.Extra) != 0 {
		t.Errorf("Extra = %+v, want nothing for keys this library names", tag.Extra)
	}
}

// TestParseSentenceWithTagBlock is the point of the feature: a tagged line
// yields a normal sentence with the block attached, not an error.
func TestParseSentenceWithTagBlock(t *testing.T) {
	s, err := ParseSentence(taggedAIS)
	if err != nil {
		t.Fatalf("ParseSentence: %v", err)
	}
	if s.Type != "VDM" {
		t.Errorf("Type = %q, want VDM", s.Type)
	}
	if s.Talker != "AI" {
		t.Errorf("Talker = %q, want AI", s.Talker)
	}
	// The payload field must be the AIS payload, with no tag block text mixed in.
	if len(s.Fields) != 6 {
		t.Fatalf("got %d fields, want 6: %q", len(s.Fields), s.Fields)
	}
	if s.Fields[0] != "1" || s.Fields[1] != "1" {
		t.Errorf("fields 0 and 1 = %q, %q, want 1, 1", s.Fields[0], s.Fields[1])
	}
	if s.Fields[4] == "" {
		t.Error("the AIS payload field is empty")
	}
	// Both checksums are verified independently.
	if s.ChecksumState != ChecksumValid {
		t.Errorf("sentence checksum state = %v, want valid", s.ChecksumState)
	}
	if !s.TagBlock.Present || !s.TagBlock.HasSource {
		t.Error("the tag block was not attached to the sentence")
	}
}

// TestTagBlockAbsent checks that an ordinary sentence reports no block rather
// than a zero one that looks present.
func TestTagBlockAbsent(t *testing.T) {
	s, err := ParseSentence("$GPGGA,161229.487,3723.2475,N,12158.3416,W,1,07,1.0,9.0,M,,,,0000*18")
	if err != nil {
		t.Fatalf("ParseSentence: %v", err)
	}
	if s.TagBlock.Present {
		t.Error("Present = true for a sentence with no tag block")
	}
	if s.TagBlock.HasSource || s.TagBlock.HasTime {
		t.Error("a zero tag block reported a field as present")
	}

	// ParseTagBlock on an untagged line consumes nothing and is not an error.
	tag, n, err := ParseTagBlock("$GPGGA,161229.487,3723.2475,N*4E")
	if err != nil {
		t.Errorf("ParseTagBlock on an untagged line: %v", err)
	}
	if n != 0 || tag.Present {
		t.Errorf("ParseTagBlock consumed %d bytes and reported present %v, want 0 and false", n, tag.Present)
	}
}

// TestTagBlockKeepsUnknownKeys covers the IEC 62320-1 variant, which uses the
// same block format with a partly different key set. Dropping the keys this
// library does not name would lose the metadata for exactly the receivers that
// send them.
func TestTagBlockKeepsUnknownKeys(t *testing.T) {
	// x is the IEC line-count key and xGy the IEC grouping key, neither of
	// which the NMEA set uses.
	line := `\xGy:1-2,x:42,zz:custom*00\!AIVDM,1,1,,A,177KQJ,0`
	tag, _, err := ParseTagBlock(line)
	if err == nil {
		t.Log("the example block's checksum does not verify, which is expected for a hand-built line")
	}
	if !tag.Present {
		t.Fatal("Present = false for a line that carries a tag block")
	}
	if len(tag.Extra) != 3 {
		t.Fatalf("Extra = %+v, want three unknown keys kept", tag.Extra)
	}
	if tag.Extra[0].Key != "xGy" || tag.Extra[0].Value != "1-2" {
		t.Errorf("Extra[0] = %+v, want xGy:1-2", tag.Extra[0])
	}
	if tag.Extra[2].Key != "zz" || tag.Extra[2].Value != "custom" {
		t.Errorf("Extra[2] = %+v, want zz:custom", tag.Extra[2])
	}
	// The generic lookup finds them, which is what a program using the IEC key
	// set needs.
	if v, ok := tag.Field("xGy"); !ok || v != "1-2" {
		t.Errorf("Field(xGy) = %q, %v; want 1-2, true", v, ok)
	}
	// And it finds the named ones too.
	if v, ok := tag.Field("n"); ok {
		t.Errorf("Field(n) = %q, true; want false: this block used the IEC key x", v)
	}
}

// TestTagBlockRejectsNonNumeric covers the numeric keys. A timestamp that is not
// a number is a fault worth reporting rather than storing as zero, because zero
// is a plausible time.
func TestTagBlockRejectsNonNumeric(t *testing.T) {
	for _, key := range []string{"c", "r", "n"} {
		line := `\` + key + `:notanumber*00\!AIVDM,1,1,,A,177KQJ,0`
		if _, _, err := ParseTagBlock(line); err == nil {
			t.Errorf("a tag block with a non-numeric %s field was accepted", key)
		}
	}
}

// TestTagBlockRejectsUnterminated covers a line that opens a block and never
// closes it, which is what a truncated read looks like.
func TestTagBlockRejectsUnterminated(t *testing.T) {
	if _, _, err := ParseTagBlock(`\s:foo,c:1241544035`); err == nil {
		t.Error("an unterminated tag block was accepted")
	}
}

// TestTagBlockChecksumIsIndependent covers the two checksums. A block can be
// wrong while the sentence is right and the other way round, so they are
// verified separately rather than as one.
func TestTagBlockChecksumIsIndependent(t *testing.T) {
	// Corrupt the block's checksum only, leaving the sentence's correct, so the
	// two verdicts differ. The raw literal ends at the closing backslash, and
	// bang supplies the '!' that starts the sentence.
	line := `\s:foo,c:1241544035*00\` + bang(aisPayloadBody)

	s, err := ParseSentence(line)
	if err != nil {
		t.Fatalf("ParseSentence: %v", err)
	}
	if s.TagBlock.ChecksumState != ChecksumInvalid {
		t.Errorf("tag block checksum state = %v, want invalid", s.TagBlock.ChecksumState)
	}
	if s.ChecksumState != ChecksumValid {
		t.Errorf("sentence checksum state = %v, want valid: the two are independent", s.ChecksumState)
	}
	// The block's fields are still readable, because a bad timestamp does not
	// make the metadata unreadable, only untrustworthy.
	if !s.TagBlock.HasSource || s.TagBlock.Source != "foo" {
		t.Error("the source was lost because the block's checksum was wrong")
	}
}

// TestTagBlockMillisecondsHeuristic covers the unit the standard leaves open.
// It says seconds or milliseconds and does not say which, so a value too large
// to be seconds is read as milliseconds and flagged.
func TestTagBlockMillisecondsHeuristic(t *testing.T) {
	seconds, _, _ := ParseTagBlock(`\c:1241544035*00\!AIVDM,1,1,,A,177,0`)
	if seconds.TimeIsMilli {
		t.Error("TimeIsMilli = true for a seconds timestamp")
	}
	milli, _, _ := ParseTagBlock(`\c:1241544035000*00\!AIVDM,1,1,,A,177,0`)
	if !milli.TimeIsMilli {
		t.Error("TimeIsMilli = false for a millisecond timestamp")
	}
	if milli.Time != 1241544035000 {
		t.Errorf("Time = %v, want the value as sent", milli.Time)
	}
}

// TestTagBlockOnBangSentence covers the encapsulated-data prefix. Every AIS
// sentence uses '!' rather than '$', so accepting only '$' would make the whole
// AIS set unparseable.
func TestTagBlockOnBangSentence(t *testing.T) {
	s, err := ParseSentence("!AIVDM,1,1,,B,177KQJ5000G?tO`K>RA1wUbN0TKH,0*5C")
	if err != nil {
		t.Fatalf("ParseSentence on a '!' sentence: %v", err)
	}
	if s.Type != "VDM" {
		t.Errorf("Type = %q, want VDM", s.Type)
	}
	if s.ChecksumState != ChecksumValid {
		t.Errorf("checksum state = %v, want valid", s.ChecksumState)
	}
}
