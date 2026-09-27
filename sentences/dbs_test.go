package sentences

import (
	"math"
	"testing"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// DBS decoder tests. DBS shares its layout with DBT and DBK; the difference is
// what the depth is measured from.

// TestPublishedDBS verifies the layout against the documented example.
func TestPublishedDBS(t *testing.T) {
	d := published[DBS](t, "SDDBS,7.8,f,2.4,M,1.3,F")

	if !d.HasFeet || d.Feet != 7.8 {
		t.Errorf("field 0 feet = %v (present %v), want 7.8", d.Feet, d.HasFeet)
	}
	if !d.HasMetres || d.Metres != 2.4 {
		t.Errorf("field 1 metres = %v (present %v), want 2.4", d.Metres, d.HasMetres)
	}
	if !d.HasFathoms || d.Fathoms != 1.3 {
		t.Errorf("field 2 fathoms = %v (present %v), want 1.3", d.Fathoms, d.HasFathoms)
	}
	if d.DataType() != "DBS" {
		t.Errorf("DataType = %q, want DBS", d.DataType())
	}
}

// TestDBSPartialUnits covers the documented case of a sensor reporting only one
// unit.
func TestDBSPartialUnits(t *testing.T) {
	d := published[DBS](t, "SDDBS,,f,22.5,M,,F")
	got, ok := d.MetresOrConverted()
	if !ok || got != 22.5 {
		t.Errorf("MetresOrConverted = %v, %v; want 22.5, true", got, ok)
	}
}

// TestDBSKeepsItsOwnFixField checks that DBS does not overwrite the depth below
// the transducer. The two differ by the transducer's height above the water, so
// reporting one as the other would be wrong by exactly that offset.
func TestDBSKeepsItsOwnFixField(t *testing.T) {
	var f nmea.Fix
	// A transducer 0.5 m below the waterline reads 0.5 m less than the surface.
	published[DBT](t, "SDDBT,,f,2.4,M,,F").ApplyFix(&f)
	published[DBS](t, "SDDBS,,f,2.9,M,,F").ApplyFix(&f)

	if !f.HasDepth || math.Abs(f.DepthMetres-2.4) > 1e-9 {
		t.Errorf("fix DepthMetres = %v, want 2.4 from the DBT", f.DepthMetres)
	}
	if !f.HasSurfaceDepth || math.Abs(f.DepthBelowSurface-2.9) > 1e-9 {
		t.Errorf("fix DepthBelowSurface = %v, want 2.9 from the DBS", f.DepthBelowSurface)
	}
}
