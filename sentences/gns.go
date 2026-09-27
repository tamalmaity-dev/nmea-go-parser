package sentences

import "github.com/tamalmaity-dev/nmea-go-parser"

// GNS is the GNSS Fix Data sentence. It is a more capable sibling of GGA:
// the same position, but with a mode indicator that names each constellation
// contributing to the solution, a satellite count that is not capped at
// twelve, and a navigational status.
//
//	$GPGNS,112257.00,3844.24011,N,00908.43828,W,AN,03,10.5,,*57
//	$GNGNS,103607.00,5327.03942,N,00214.42462,W,ANNN,07,1.2,71.0,50.5,,,S*3F
//
// Field layout:
//
//	 0 UTC time          hhmmss.sss
//	 1 latitude          ddmm.mmmm
//	 2 latitude hemi     N or S
//	 3 longitude         dddmm.mmmm
//	 4 longitude hemi    E or W
//	 5 mode indicator    1 to 6 characters, one per constellation
//	 6 satellites in use 00-99, not capped at twelve as in GGA
//	 7 HDOP              horizontal dilution of precision
//	 8 altitude          metres above mean sea level
//	 9 geoid separation  metres, MSL to ellipsoid
//	10 DGPS age          seconds, optional
//	11 DGPS station id   0000-1023, optional
//	12 nav status        S safe, C caution, U unsafe, V not valid, NMEA 4.10
//
// The mode indicator is the distinguishing field and the one most often
// misread: it is a variable-length string, one character per constellation,
// not a single flag. "AN" means GPS autonomous plus a second constellation
// not yet in use. Per-character meaning is in ModePerConstellation.
type GNS struct {
	Base
	// Latitude, Longitude, HasPosition, and the raw ddmm.mmmm forms
	// come from the embedded LatLon.
	LatLon
	// UTC is the time of day the fix was taken.
	UTC nmea.TOD
	// ModeIndicator is the raw per-constellation string from field 5.
	ModeIndicator string
	// Modes splits ModeIndicator into per-constellation readings, in the
	// order GPS, GLONASS, Galileo, BeiDou, QZSS, NavIC, skipping any that
	// the receiver did not report.
	Modes []ConstellationMode
	// SatellitesUsed is the count in use, which unlike GGA's is not capped
	// at twelve.
	SatellitesUsed     int
	HasSatellitesUsed  bool
	HDOP               float64
	HasHDOP            bool
	Altitude           float64
	HasAltitude        bool
	GeoidSeparation    float64
	HasGeoidSeparation bool
	DGPSAge            float64
	HasDGPSAge         bool
	DGPSStationID      int
	HasDGPSStationID   bool
	// NavStatus is the NMEA 4.10 navigational status.
	NavStatus    nmea.NavStatus
	HasNavStatus bool
}

// ConstellationMode is one constellation's contribution to a GNS solution.
type ConstellationMode struct {
	// Constellation is the system, derived from the character position in
	// the mode string.
	Constellation nmea.Constellation
	// Mode is how that constellation is being used.
	Mode Mode
	// Raw is the original character, kept because a receiver that emits an
	// undocumented letter should still be reportable.
	Raw byte
}

// Mode is a per-constellation fix mode: how a system is being used in the
// solution.
type Mode int

const (
	ModeNone Mode = iota // no fix from this system
	ModeAutonomous
	ModeDifferential
	ModeEstimated // dead reckoning
	ModeRTKFloat
	ModeManual
	ModePrecise
	ModeRTKFixed
	ModeSimulator
	ModeUnknownMode
)

func (m Mode) String() string {
	switch m {
	case ModeNone:
		return "no fix"
	case ModeAutonomous:
		return "autonomous"
	case ModeDifferential:
		return "differential"
	case ModeEstimated:
		return "estimated"
	case ModeRTKFloat:
		return "RTK float"
	case ModeManual:
		return "manual"
	case ModePrecise:
		return "precise"
	case ModeRTKFixed:
		return "RTK fixed"
	case ModeSimulator:
		return "simulator"
	default:
		return "unknown"
	}
}

// Fixed reports whether the mode yields a usable position from that system.
func (m Mode) Fixed() bool {
	switch m {
	case ModeAutonomous, ModeDifferential, ModeRTKFloat, ModePrecise, ModeRTKFixed:
		return true
	default:
		return false
	}
}

// ParseMode converts one character of the GNS mode indicator.
func ParseMode(c byte) Mode {
	switch c {
	case 'A':
		return ModeAutonomous
	case 'D':
		return ModeDifferential
	case 'E':
		return ModeEstimated
	case 'F':
		return ModeRTKFloat
	case 'M':
		return ModeManual
	case 'N':
		return ModeNone
	case 'P':
		return ModePrecise
	case 'R':
		return ModeRTKFixed
	case 'S':
		return ModeSimulator
	default:
		return ModeUnknownMode
	}
}

// constellationOrder is the order the characters of a GNS mode string
// appear in, which is fixed by the standard and not by any talker.
var constellationOrder = []nmea.Constellation{
	nmea.ConstellationGPS,
	nmea.ConstellationGLONASS,
	nmea.ConstellationGalileo,
	nmea.ConstellationBeiDou,
	nmea.ConstellationQZSS,
	nmea.ConstellationNavIC,
}

// ModePerConstellation splits the mode string into per-system readings.
//
// The string is positional: the first character is GPS, the second
// GLONASS, and so on. A receiver that uses only GPS and GLONASS emits "AN",
// not "AN____"; trailing systems are simply absent, so the string is read
// until it runs out rather than being padded.
func (g *GNS) ModePerConstellation() []ConstellationMode {
	out := make([]ConstellationMode, 0, len(g.ModeIndicator))
	for i := 0; i < len(g.ModeIndicator) && i < len(constellationOrder); i++ {
		c := g.ModeIndicator[i]
		out = append(out, ConstellationMode{
			Constellation: constellationOrder[i],
			Mode:          ParseMode(c),
			Raw:           c,
		})
	}
	return out
}

type gns struct{}

func (gns) Formatter() string { return "GNS" }

func (gns) Decode(s nmea.Sentence) (any, error) {
	// Fields 0 to 4 are the time and position. A GNS with no position is a
	// receiver with no fix, which is normal and must not be an error.
	if !s.HasFields(5) {
		return nil, needFields(s, 5)
	}
	out := GNS{Base: newBase(s)}

	var err error
	if out.UTC, err = s.Time(0); err != nil {
		return out, err
	}
	out.LatLon = latLonFrom(s, 1)

	out.ModeIndicator = text(s.Field(5))
	out.Modes = out.ModePerConstellation()

	if out.SatellitesUsed, out.HasSatellitesUsed, err = optionalInt(s, 6); err != nil {
		return out, err
	}
	if out.HDOP, out.HasHDOP, err = optionalFloat(s, 7); err != nil {
		return out, err
	}
	if out.Altitude, out.HasAltitude, err = optionalFloat(s, 8); err != nil {
		return out, err
	}
	if out.GeoidSeparation, out.HasGeoidSeparation, err = optionalFloat(s, 9); err != nil {
		return out, err
	}
	if out.DGPSAge, out.HasDGPSAge, err = optionalFloat(s, 10); err != nil {
		return out, err
	}
	if out.DGPSStationID, out.HasDGPSStationID, err = optionalInt(s, 11); err != nil {
		return out, err
	}
	if !s.Blank(12) {
		if out.NavStatus, err = nmea.ParseNavStatus(s.Field(12)); err != nil {
			return out, err
		}
		out.HasNavStatus = true
	}
	return out, nil
}

// NMEASupports implements nmea.FeatureEvidence.
func (g GNS) NMEASupports() []nmea.Feature {
	if g.HasNavStatus {
		return nmea.FeatureOnly(nmea.FeatureNavStatus)
	}
	return nil
}

// ApplyFix folds the GNS into the fix state.
//
// GNS is the most informative position sentence a receiver emits: it carries
// altitude, HDOP, the satellite count, and a navigational status, so it can
// establish more of the fix than GGA does. It reports no fix-quality
// indicator, so it leaves Quality to GGA.
func (g GNS) ApplyFix(f *nmea.Fix) {
	if !applyPosition(f, g.LatLon) {
		return
	}
	if g.HasSatellitesUsed {
		f.SatellitesUsed, f.HasSatellitesUsed = g.SatellitesUsed, true
	}
	if g.HasHDOP {
		f.HDOP, f.HasHDOP = g.HDOP, true
	}
	if g.HasAltitude {
		f.Altitude, f.HasAltitude = g.Altitude, true
	}
	if g.HasGeoidSeparation {
		f.GeoidHeight, f.HasGeoidHeight = g.GeoidSeparation, true
	}
	if g.UTC.Available {
		f.TimeOfDay = g.UTC
	}
	if g.HasNavStatus {
		f.NavStatus, f.HasNavStatus = g.NavStatus, true
	}

	// A position is only valid if at least one constellation contributed a
	// real fix, and the navigational status permits it. GNS can report a
	// position alongside mode "N" meaning no system has a solution, which is
	// a stale coordinate rather than a current one.
	f.Valid = g.positionIsNavigable()
}

// positionIsNavigable reports whether the mode string and the navigational
// status together permit using this fix.
func (g *GNS) positionIsNavigable() bool {
	if g.HasNavStatus && !g.NavStatus.Navigable() {
		return false
	}
	// With no mode string at all, fall back on the presence of a position,
	// which is the older NMEA 2.x behaviour.
	if g.ModeIndicator == "" {
		return true
	}
	for _, m := range g.Modes {
		if m.Mode.Fixed() {
			return true
		}
	}
	return false
}
