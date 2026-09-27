package sentences

// SpeedUnit is the unit letter on a speed field. Sentences differ in which
// units they permit: MWV allows km/h, m/s, and knots, while WCV defines only
// knots. Modelling it as a type rather than a bare string makes the difference
// visible in the API.
type SpeedUnit int

const (
	SpeedUnitUnknown SpeedUnit = iota
	SpeedUnitKnots
	SpeedUnitKmh
	SpeedUnitMps
)

func (u SpeedUnit) String() string {
	switch u {
	case SpeedUnitKnots:
		return "knots"
	case SpeedUnitKmh:
		return "km/h"
	case SpeedUnitMps:
		return "m/s"
	default:
		return "unknown units"
	}
}

// ToKnots converts a speed in the given unit to knots. An unknown unit is
// returned unchanged, since guessing would silently misreport a measurement.
func (u SpeedUnit) ToKnots(v float64) float64 {
	switch u {
	case SpeedUnitKmh:
		return v / 1.852
	case SpeedUnitMps:
		return v / 0.514444
	case SpeedUnitKnots, SpeedUnitUnknown:
		return v
	default:
		return v
	}
}

// ToMetresPerSecond converts a speed in the given unit to m/s.
func (u SpeedUnit) ToMetresPerSecond(v float64) float64 {
	switch u {
	case SpeedUnitKnots:
		return knotsToMps(v)
	case SpeedUnitKmh:
		return v / 3.6
	case SpeedUnitMps, SpeedUnitUnknown:
		return v
	default:
		return v
	}
}

// ParseSpeedUnit converts a unit letter. A blank field yields
// SpeedUnitUnknown with no error, since the unit only means something next to
// a number.
func ParseSpeedUnit(s string) (SpeedUnit, error) {
	switch text(s) {
	case "":
		return SpeedUnitUnknown, nil
	case "N", "n":
		return SpeedUnitKnots, nil
	case "K", "k":
		return SpeedUnitKmh, nil
	case "M", "m":
		return SpeedUnitMps, nil
	default:
		return SpeedUnitUnknown, errRange("speed unit", "unit", 0, "N, K, or M")
	}
}
