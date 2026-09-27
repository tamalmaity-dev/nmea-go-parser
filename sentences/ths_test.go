package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TestPublishedTHS verifies the true heading and mode layout. gpsd does not
// document THS, so the sentence is checked against the field list used by
// go-nmea and by the receivers that emit it:
//
//	$GPTHS,338.01,A*0E
func TestPublishedTHS(t *testing.T) {
	th := published[THS](t, "GPTHS,338.01,A")

	if !th.Heading.HasDegrees {
		t.Fatal("HasDegrees = false for a sentence carrying a heading")
	}
	if !near(th.Heading.Degrees, 338.01, 1e-9) {
		t.Errorf("field 0 heading = %v, want 338.01", th.Heading.Degrees)
	}
	// THS is a true heading by definition, so the reference is implied
	// rather than carried.
	if !th.Heading.True {
		t.Error("True = false, want true: THS is defined as a true heading")
	}
	if th.Mode != nmea.StatusFlag('A') {
		t.Errorf("field 1 mode = %q, want A", th.Mode)
	}
	if got := th.HeadingMode(); got != "autonomous" {
		t.Errorf("HeadingMode = %q, want autonomous", got)
	}
}

// TestTHSModeAlphabet checks the mode indicator against its own alphabet. The
// letters are A, E, M, S, and V, which is a different set from the FAA mode
// used by GGA, so a decoder that reuses the GGA table would misreport every
// THS it sees.
func TestTHSModeAlphabet(t *testing.T) {
	for field, want := range map[string]string{
		"A": "autonomous",
		"E": "estimated",
		"M": "manual",
		"S": "simulated",
		"V": "not valid",
	} {
		th := published[THS](t, "GPTHS,338.01,"+field)
		if th.Mode != nmea.StatusFlag(field[0]) {
			t.Errorf("mode %q decoded as %q, want %q", field, th.Mode, field)
		}
		if got := th.HeadingMode(); got != want {
			t.Errorf("HeadingMode for %q = %q, want %q", field, got, want)
		}
	}
}

// TestTHSRejectsOutOfRangeHeading checks the range guard. A heading is 0 to
// 360 degrees, so a larger value is a decode failure rather than a heading
// that gets clamped into range.
func TestTHSRejectsOutOfRangeHeading(t *testing.T) {
	if err := publishedErr(t, "GPTHS,361.0,A"); err == nil {
		t.Error("decoding a THS heading of 361 degrees succeeded, want an error")
	}
}
