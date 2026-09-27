package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// XTE published-example checks: the field layout is verified against
// the example sentence in the NMEA documentation.

// TestPublishedXTE verifies the cross-track layout with its separate
// direction field, which is not sign-applied on the wire.
func TestPublishedXTE(t *testing.T) {
	// Published example, which has no magnitude at all.
	x := published[XTE](t, "GPXTE,V,V,,,N,S")

	// Field 0: the general warning flag, V. Field 1: the lock warning, V.
	if x.Status != nmea.StatusInvalid {
		t.Errorf("field 0 status = %v, want invalid", x.Status)
	}
	if x.LockStatus != nmea.StatusInvalid {
		t.Errorf("field 1 lock status = %v, want invalid", x.LockStatus)
	}
	// Field 2: the magnitude, blank. Field 3: the direction, blank.
	if x.HasCrossTrack {
		t.Errorf("field 2 cross-track = %v, want absent", x.CrossTrack)
	}
	if x.Steer != nmea.SideUnknown {
		t.Errorf("field 3 direction = %v, want unknown", x.Steer)
	}
	// Field 4: the unit, N. Field 5: the FAA mode, S.
	if x.Unit != UnitNauticalMiles {
		t.Errorf("field 4 unit = %v, want nautical miles", x.Unit)
	}
	if x.Mode != "S" {
		t.Errorf("field 5 mode = %q, want S", x.Mode)
	}

	// A populated example, to check the magnitude and direction.
	populated := published[XTE](t, "GPXTE,A,A,0.67,L,N")
	if !near(populated.CrossTrack, 0.67, 1e-9) {
		t.Errorf("field 2 cross-track = %v, want 0.67", populated.CrossTrack)
	}
	if populated.Steer != nmea.SideLeft {
		t.Errorf("field 3 direction = %v, want left", populated.Steer)
	}
}
