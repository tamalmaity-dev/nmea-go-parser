package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// TLL is the Target Latitude and Longitude sentence: where a tracked target is,
// paired with the TTM that describes its motion.
//
//	$GPTLL,31,4951.42,N,01211.75,W,WHALE,123519,A*32
//	$GPTLL,31,4807.038,N,01131.000,E,TGT1,220516,V,A*3D
//
// Field layout:
//
//	0 target number, 0 to 99
//	1 latitude, ddmm.mm     2 N or S
//	3 longitude, dddmm.mm   4 E or W
//	5 target name
//	6 UTC of the data
//	7 status, L lost, Q acquiring, T tracking
//	8 R for a reference target, blank otherwise
//
// The target number is what ties this to TTM and TLB, and it is the only way to
// know which motion figures belong to which position. A program tracking
// targets has to join on it.
//
// The number of digits past the decimal point is equipment dependent, so the
// coordinate is kept in the raw form alongside the decimal degrees. A receiver
// sending two decimal places and one sending four would otherwise produce
// positions that differ by a hundred metres for what is the same target.
type TLL struct {
	Base
	// Number is the tracker's label for this target.
	Number    int
	HasNumber bool

	// Latitude, Longitude, HasPosition, and the raw forms come from the
	// embedded LatLon. This is the target's position, not the vessel's, so it
	// must never reach the fix.
	LatLon

	// Name identifies the target for a display.
	Name string
	// UTC is when the position was observed. A tracker repeats a target's
	// position without re-measuring it, so this is what says how stale the
	// position is.
	UTC    nmea.TOD
	HasUTC bool
	// Status is L for a lost target, Q while acquiring, and T while tracking.
	Status string
	// Reference marks the one target used as the reference for relative
	// motion, which is a single target by definition.
	Reference bool
}

type tll struct{}

func (tll) Formatter() string { return "TLL" }

func (tll) Decode(s nmea.Sentence) (any, error) {
	out := TLL{Base: newBase(s)}
	// The target number is what TTM and TLB refer back to, so it is required.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}

	var err error
	if out.Number, out.HasNumber, err = optionalInt(s, 0); err != nil {
		return out, err
	}
	// A tracker that has lost a target keeps reporting its last known position,
	// so a blank position is a normal state rather than a fault. The status
	// field says which, and the caller checks that.
	if !s.Blank(1) || !s.Blank(3) {
		if out.LatLon, err = latLonOptional(s, 1); err != nil {
			return out, err
		}
	}
	out.Name = text(s.Field(5))
	if out.UTC, err = s.Time(6); err == nil {
		out.HasUTC = true
	}
	out.Status = text(s.Field(7))
	out.Reference = text(s.Field(8)) == "R"
	return out, nil
}
