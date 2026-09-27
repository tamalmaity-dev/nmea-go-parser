package sentences

import (
	"errors"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TestPublishedHDT verifies the heading layout against the example printed in
// the NMEA documentation:
//
//	$GPHDT,274.07,T*03
func TestPublishedHDT(t *testing.T) {
	h := published[HDT](t, "GPHDT,274.07,T")

	if !h.Heading.HasDegrees {
		t.Fatal("HasDegrees = false for a sentence carrying a heading")
	}
	if !near(h.Heading.Degrees, 274.07, 1e-9) {
		t.Errorf("field 0 heading = %v, want 274.07", h.Heading.Degrees)
	}
	// Field 1 is the T reference. A true heading and a magnetic one differ by
	// the local variation, so the flag decides which number it is.
	if !h.Heading.True {
		t.Error("field 1 reference: True = false, want true for a T reference")
	}
}

// TestHDTMagneticReferenceIsNotTrue checks the property that matters most
// about the reference flag. HDT is defined as a true heading, so a receiver
// sending M is not following the format, but the safe outcome is to report
// the value as magnetic rather than to reject it or to accept it as true. A
// magnetic heading read as a true one is a silent error of 10 to 20 degrees,
// and True is the flag that prevents it.
func TestHDTMagneticReferenceIsNotTrue(t *testing.T) {
	h := published[HDT](t, "GPHDT,274.07,M")

	if !h.Heading.HasDegrees || !near(h.Heading.Degrees, 274.07, 1e-9) {
		t.Fatalf("heading = %v (present %v), want 274.07", h.Heading.Degrees, h.Heading.HasDegrees)
	}
	if h.Heading.True {
		t.Error("True = true for an M reference, want false")
	}
}

// TestHDTBlankHeading checks the required field. The heading is the entire
// sentence, so a blank one leaves nothing to deliver. The error has to name
// the empty field rather than a field count, because the field is present and
// only its value is missing, and a caller chasing a truncation that did not
// happen wastes a lot of time.
func TestHDTBlankHeading(t *testing.T) {
	err := publishedErr(t, "GPHDT,,")
	if err == nil {
		t.Fatal("decoding a blank HDT heading succeeded, want an error")
	}
	if !errors.Is(err, nmea.ErrEmptyField) {
		t.Errorf("decoding a blank HDT heading = %v, want ErrEmptyField", err)
	}
	if errors.Is(err, nmea.ErrFieldCount) {
		t.Error("a blank heading was reported as a field count problem: the field is present")
	}
}
