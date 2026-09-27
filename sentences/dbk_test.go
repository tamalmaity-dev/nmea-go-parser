package sentences

import (
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// DBK decoder tests. DBK is obsolete in the standard but still on the wire,
// and it carries the one depth figure a caller cannot derive from the others.

// TestPublishedDBK verifies the layout against the documented example.
func TestPublishedDBK(t *testing.T) {
	d := published[DBK](t, "SDDBK,7.8,f,2.4,M,1.3,F")

	if !d.HasFeet || d.Feet != 7.8 {
		t.Errorf("field 0 feet = %v (present %v), want 7.8", d.Feet, d.HasFeet)
	}
	if !d.HasMetres || d.Metres != 2.4 {
		t.Errorf("field 1 metres = %v (present %v), want 2.4", d.Metres, d.HasMetres)
	}
	if !d.HasFathoms || d.Fathoms != 1.3 {
		t.Errorf("field 2 fathoms = %v (present %v), want 1.3", d.Fathoms, d.HasFathoms)
	}
	if d.DataType() != "DBK" {
		t.Errorf("DataType = %q, want DBK", d.DataType())
	}
}

// TestDBKSetsKeelDepth checks the one thing DBK is for. It reports the depth
// beneath the hull directly, so a program does not have to know the transducer
// offset to get the figure that decides whether the next grounding happens.
func TestDBKSetsKeelDepth(t *testing.T) {
	var f nmea.Fix
	published[DBK](t, "SDDBK,,f,2.4,M,,F").ApplyFix(&f)

	if !f.HasKeelDepth || f.DepthBelowKeel != 2.4 {
		t.Errorf("fix DepthBelowKeel = %v (present %v), want 2.4", f.DepthBelowKeel, f.HasKeelDepth)
	}
	// It is a keel figure, so it must not be filed as a transducer or surface
	// depth as well.
	if f.HasDepth {
		t.Error("DBK also set DepthMetres, which measures from the transducer")
	}
	if f.HasSurfaceDepth {
		t.Error("DBK also set DepthBelowSurface, which measures from the waterline")
	}
}

// TestDBKPartialUnits covers a receiver reporting only one unit.
func TestDBKPartialUnits(t *testing.T) {
	d := published[DBK](t, "SDDBK,7.8,f,,M,,F")
	got, ok := d.MetresOrConverted()
	if !ok {
		t.Fatal("MetresOrConverted reported nothing for a feet-only DBK")
	}
	if got < 2.37 || got > 2.38 {
		t.Errorf("MetresOrConverted = %v, want about 2.377", got)
	}

	var f nmea.Fix
	d.ApplyFix(&f)
	if !f.HasKeelDepth {
		t.Error("a feet-only DBK did not reach the fix")
	}
}
