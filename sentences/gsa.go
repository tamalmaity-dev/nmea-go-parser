package sentences

import (
	"fmt"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// GSA is the DOP and Active Satellites sentence. It names the satellites
// actually used in the solution and reports the three dilution-of-precision
// figures, which together describe the geometry of the constellation and are
// the usual basis for rejecting a fix.

//------------------------------------------------------------------------------------------
//	$GPGSA,A,3,04,05,,09,12,,,24,,,,,2.5,1.3,2.1*33
//------------------------------------------------------------------------------------------

// Field layout:
//
//	 0 mode             A automatic, M manual
//	 1 fix type         1 no fix, 2 2D, 3 3D
//	 2-13 satellite ids up to 12 PRNs, blank for unused slots
//	14 PDOP             position dilution
//	15 HDOP             horizontal dilution
//	16 VDOP             vertical dilution
//	17 system id        GNSS system, NMEA 4.10 and later, optional
//
// Field 1 is the sentence's own opinion about fix quality, independent of
// GGA. They usually agree; when they do not, GGA is the one to trust,
// because it is what a receiver is required to report consistently.
//
// Field 17 is optional in practice even on NMEA 4.10 receivers: the same
// model has been observed emitting GSA with and without it in the same
// session, so its absence is not an error. Some receivers put a signal id
// there instead when the talker already identifies the constellation, which
// is why it is exposed as a raw value alongside the named SystemID.
type GSA struct {
	Base
	// Mode is A for automatic selection of the 3D solution, M for manual.
	Mode nmea.StatusFlag
	// FixType is 1 for no fix, 2 for 2D, 3 for 3D.
	FixType    int
	HasFixType bool
	// SatelliteIDs are the PRNs used in the solution, in slot order, with
	// blank slots omitted.
	SatelliteIDs []int
	// PDOP, HDOP, and VDOP are the three dilution-of-precision figures.
	// Lower is better; PDOP combines all three, HDOP covers the horizontal
	// plane, and VDOP is the vertical component that gates altitude.
	// Receivers that lack a 3D solution often send PDOP and HDOP but leave
	// VDOP blank, hence the flags.
	PDOP    float64
	HasPDOP bool
	HDOP    float64
	HasHDOP bool
	VDOP    float64
	HasVDOP bool
	// SystemID is the GNSS system, NMEA 4.10 and later.
	SystemID    nmea.SystemID
	HasSystemID bool
	// TrailingField is the raw value of field 17. It is kept because the
	// meaning of that field differs between receivers: a system id on a
	// $GNGS sentence, a signal id when the talker already names the
	// constellation, or a build artefact on some older models.
	TrailingField    int
	HasTrailingField bool
}

// DimensionalFix describes how many dimensions the solution constrains.
type DimensionalFix int

const (
	NoFix    DimensionalFix = 1
	Degrees2 DimensionalFix = 2
	Degrees3 DimensionalFix = 3
)

func (d DimensionalFix) String() string {
	switch d {
	case NoFix:
		return "no fix"
	case Degrees2:
		return "2D"
	case Degrees3:
		return "3D"
	default:
		return "unknown"
	}
}

// Dimensions returns the GSA fix type as a named value.
func (g *GSA) Dimensions() DimensionalFix { return DimensionalFix(g.FixType) }

type gsa struct{}

func (gsa) Formatter() string { return "GSA" }

func (gsa) Decode(s nmea.Sentence) (any, error) {
	// The three DOP fields close the sentence, so a short one has no fix
	// quality information in it.
	if !s.HasFields(3) {
		return nil, needFields(s, 3)
	}
	out := GSA{Mode: nmea.ParseStatusFlag(s.Field(0))}

	fixType, present, err := optionalInt(s, 1)
	if err != nil {
		return out, err
	}
	if present {
		if fixType < 1 || fixType > 3 {
			return out, fmt.Errorf("sentences: GSA fix type %d must be 1, 2, or 3: %w",
				fixType, nmea.ErrFieldRange)
		}
		out.FixType, out.HasFixType = fixType, true
	}

	// Satellite slots 2 to 13, blank where unused. Stopping at the first
	// blank would be wrong: receivers pad from the right but some fill
	// gaps, so every slot is examined. The slice is left nil when no slot is
	// used, which is what a receiver with no fix sends, rather than
	// allocating an empty one on every such sentence.
	var ids []int
	for i := 2; i < 14; i++ {
		if s.Blank(i) {
			continue
		}
		id, err := s.Int(i)
		if err != nil {
			return out, err
		}
		ids = append(ids, id)
	}
	out.SatelliteIDs = ids

	if out.PDOP, out.HasPDOP, err = optionalFloat(s, 14); err != nil {
		return out, err
	}
	if out.HDOP, out.HasHDOP, err = optionalFloat(s, 15); err != nil {
		return out, err
	}
	if out.VDOP, out.HasVDOP, err = optionalFloat(s, 16); err != nil {
		return out, err
	}

	// Field 17 arrived in NMEA 4.10 and is optional even there. Its meaning
	// depends on the talker: on a $GNGSA it is the system id, while on a
	// $GPGSA the talker already says "GPS" and some receivers put a signal
	// id there instead. Both are recorded, and the named SystemID is set
	// from the talker when the raw value does not parse as one.
	if v, present, err := optionalInt(s, 17); err != nil {
		return out, err
	} else if present {
		out.TrailingField, out.HasTrailingField = v, true
		if id, ok := nmea.ParseSystemID(s.Field(17)); ok {
			out.SystemID, out.HasSystemID = id, true
		} else if sys := nmea.TalkerSystemID(s.Talker); sys != nmea.SystemIDNone {
			out.SystemID, out.HasSystemID = sys, true
		}
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence: a GSA carrying a trailing
// system id proves the receiver speaks NMEA 4.10 or later.
func (g GSA) NMEASupports() []nmea.Feature {
	if g.HasTrailingField {
		return nmea.FeaturesSystemAndSignal()
	}
	return nil
}

// ApplyFix folds the GSA into the fix state. GSA contributes the satellite
// count and the dilution figures, and nothing else; it carries no position,
// so it never invalidates a fix.
func (g GSA) ApplyFix(f *nmea.Fix) {
	// A multi-GNSS receiver sends one GSA per system for each fix, so the
	// satellite list is recorded per system and merged. The system id is
	// preferred over the talker because a multi-GNSS stream uses the GN talker
	// for every GSA, which names no system, while the 4.10 system id does.
	// SatellitesUsed is derived from the merged list rather than taken from
	// this sentence's length, so the count and the per-system breakdown cannot
	// disagree.
	constellation := nmea.TalkerConstellation(g.Talker())
	if g.HasSystemID {
		if c := g.SystemID.Constellation(); c != nmea.ConstellationUnknown {
			constellation = c
		}
	}
	f.SetSatelliteUse(constellation, g.SystemID, g.SatelliteIDs)

	if g.HasPDOP {
		f.PDOP, f.HasPDOP = g.PDOP, true
	}
	if g.HasHDOP {
		f.HDOP, f.HasHDOP = g.HDOP, true
	}
	if g.HasVDOP {
		f.VDOP, f.HasVDOP = g.VDOP, true
	}
}
