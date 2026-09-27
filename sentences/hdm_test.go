package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// HDM decoder tests.

// TestPublishedHDM verifies the magnetic heading layout.
func TestPublishedHDM(t *testing.T) {
	h := published[HDM](t, "HCHDM,123.4,M")

	if !h.HasHeading || h.HeadingDegrees != 123.4 {
		t.Errorf("field 0 heading = %v (present %v), want 123.4", h.HeadingDegrees, h.HasHeading)
	}
	if h.DataType() != "HDM" {
		t.Errorf("DataType = %q, want HDM", h.DataType())
	}
}

// TestHDMMagneticIsNotTrue is the point of the sentence. A magnetic heading
// differs from a true one by the local variation, so storing it with the true
// flag set would put the vessel tens of degrees off, silently, in the direction
// that matters most.
func TestHDMMagneticIsNotTrue(t *testing.T) {
	var f nmea.Fix
	published[HDM](t, "HCHDM,123.4,M").ApplyFix(&f)

	if !f.HasHeading {
		t.Fatal("ApplyFix set no heading at all")
	}
	if f.HeadingDegrees != 123.4 {
		t.Errorf("HeadingDegrees = %v, want 123.4", f.HeadingDegrees)
	}
	if f.HeadingTrue {
		t.Error("HeadingTrue = true for a magnetic heading, which would be wrong by the local variation")
	}
}

// TestHDMUpdatesTheFix checks that a later true heading is not left marked
// magnetic, and the reverse. A compass that is switched over must not leave a
// stale flag behind.
func TestHDMUpdatesTheFix(t *testing.T) {
	var f nmea.Fix
	f.HeadingDegrees, f.HasHeading, f.HeadingTrue = 45, true, true

	published[HDM](t, "HCHDM,123.4,M").ApplyFix(&f)
	if f.HeadingTrue {
		t.Error("HeadingTrue stayed true after a magnetic heading arrived")
	}
}

// TestHDMBlankHeading covers a compass with no reading, which several send as a
// blank field rather than staying quiet.
func TestHDMBlankHeading(t *testing.T) {
	h := published[HDM](t, "HCHDM,,M")
	if h.HasHeading {
		t.Error("HasHeading = true for a blank heading field")
	}

	var f nmea.Fix
	h.ApplyFix(&f)
	if f.HasHeading {
		t.Error("a blank HDM set a heading on the fix")
	}
}

// TestHDMRejectsOutOfRange checks the range guard. A compass reporting 400
// degrees is broken, and folding that into the fix would leave a vessel with a
// heading that no arithmetic downstream can use.
func TestHDMRejectsOutOfRange(t *testing.T) {
	if err := publishedErr(t, "HCHDM,361.0,M"); err == nil {
		t.Error("decoding a heading of 361 degrees succeeded, want an error")
	}
	if err := publishedErr(t, "HCHDM,-1.0,M"); err == nil {
		t.Error("decoding a heading of -1 degrees succeeded, want an error")
	}
	// The boundaries are legal.
	for _, ok := range []string{"HCHDM,0.0,M", "HCHDM,360.0,M"} {
		if err := publishedErr(t, ok); err != nil {
			t.Errorf("decoding %q failed: %v", ok, err)
		}
	}
}

// TestHDMTruncated covers a sentence with no field at all.
func TestHDMTruncated(t *testing.T) {
	if err := publishedErr(t, "HCHDM"); err == nil {
		t.Error("decoding an HDM with no fields succeeded, want an error")
	}
}
