package sentences

import (
	"fmt"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// DistanceUnit is the N/K unit flag used by the guidance sentences. It
// appears on its own or trailing a number, and getting it wrong turns a
// cross-track error into a value off by a factor of 1.852, so it is parsed
// explicitly rather than assumed.
type DistanceUnit int

const (
	// UnitUnknown means the unit flag was absent or unrecognised.
	UnitUnknown DistanceUnit = iota
	// UnitNauticalMiles is the N flag.
	UnitNauticalMiles
	// UnitKilometres is the K flag.
	UnitKilometres
)

func (u DistanceUnit) String() string {
	switch u {
	case UnitNauticalMiles:
		return "nautical miles"
	case UnitKilometres:
		return "kilometres"
	default:
		return "unknown units"
	}
}

// ToNauticalMiles converts a distance in the given unit to nautical miles,
// which is what the library stores internally. An unknown unit is returned
// unchanged, because silently assuming nautical miles would corrupt a
// kilometre reading.
func (u DistanceUnit) ToNauticalMiles(d float64) float64 {
	switch u {
	case UnitKilometres:
		return d / 1.852
	case UnitNauticalMiles, UnitUnknown:
		return d
	default:
		return d
	}
}

// ToMetres converts a distance in the given unit to metres. An unknown unit
// is treated as nautical miles, which is what a blank N/K flag means in
// practice: every receiver defaults to nautical miles.
func (u DistanceUnit) ToMetres(d float64) float64 {
	if u == UnitKilometres {
		return d * 1000
	}
	return d * 1852
}

// ParseDistanceUnit converts an "N" or "K" field. A blank field yields
// UnitUnknown with no error, since the unit is only meaningful alongside a
// number.
func ParseDistanceUnit(s string) (DistanceUnit, error) {
	switch s {
	case "":
		return UnitUnknown, nil
	case "N", "n":
		return UnitNauticalMiles, nil
	case "K", "k":
		return UnitKilometres, nil
	default:
		return UnitUnknown, fmt.Errorf("%w: distance unit %q must be N or K",
			nmea.ErrFieldValue, s)
	}
}
