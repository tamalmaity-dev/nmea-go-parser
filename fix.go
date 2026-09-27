package nmea

import (
	"fmt"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Fix is the merged, always-current picture of where the receiver thinks it is.
// It is the answer to "just give me the position": each sentence type
// fills in the fields it owns and leaves the rest untouched, so a single
// Fix can be polled at any moment without tracking GGA versus RMC.

// Every optional value has a companion Has* flag, because a blank NMEA
// field means "not available" and is meaningfully different from zero.
// Latitude of exactly 0.0 and longitude of exactly 0.0 are real positions
// in the Gulf of Guinea, so a sentinel float is not an option here.

type Fix struct {
	// Valid is true when the receiver last reported a usable position.
	// It reflects NavigationStatus and FixQuality, so it goes false on a
	// "V" status even if earlier sentences carried coordinates.
	Valid bool

	// Latitude and Longitude are signed decimal degrees. Positive
	// latitude is north, positive longitude is east.
	Latitude  float64
	Longitude float64
	// HasPosition is true once any sentence has supplied coordinates.
	HasPosition bool

	// Altitude is height above mean sea level in metres, from GGA.
	Altitude    float64
	HasAltitude bool

	// GeoidHeight is the geoid separation in metres from GGA. MSL altitude
	// above the ellipsoid is Altitude + GeoidHeight.
	GeoidHeight    float64
	HasGeoidHeight bool

	// SpeedKnots is ground speed over water in knots. SpeedKmh and
	// SpeedMps are the same value in other units, for convenience.
	SpeedKnots float64
	SpeedKmh   float64
	SpeedMps   float64
	HasSpeed   bool

	// CourseDegrees is the course over ground, degrees true, 0..360.
	CourseDegrees float64
	HasCourse     bool

	// HeadingDegrees is the vessel's heading, which is the direction it is
	// pointing rather than the direction it is travelling. HeadingTrue says
	// which; a false value means magnetic, and a program must not treat the
	// two as interchangeable.
	HeadingDegrees float64
	HasHeading     bool
	HeadingTrue    bool

	// WaterSpeedKnots is speed through the water, as opposed to the ground
	// speed in SpeedKnots. The difference between the two is the current.
	WaterSpeedKnots float64
	WaterSpeedKmh   float64
	HasWaterSpeed   bool

	// Set is the direction the water is setting towards, degrees true, and
	// DriftKnots is the rate of the current.
	Set        float64
	HasSet     bool
	DriftKnots float64
	HasDrift   bool

	// RateOfTurn is degrees per minute, positive to starboard.
	RateOfTurn    float64
	HasRateOfTurn bool

	// RudderAngle is degrees, negative to port.
	RudderAngle float64
	HasRudder   bool

	// Environment measurements. These are not part of a position fix, but a
	// receiver that supplies them is usually reporting on a vessel, and
	// carrying them here means a program can log everything in one place.
	WaterTempC   float64
	HasWaterTemp bool
	// AirTempC is the air temperature, from MTA. It is separate from
	// WaterTempC because the two are independent sensors and a program asking
	// for the sea temperature must not be handed the air temperature.
	AirTempC         float64
	HasAirTemp       bool
	WindSpeedKnots   float64
	HasWindSpeed     bool
	WindDirection    float64
	HasWindDirection bool
	// DepthMetres is the water depth below the transducer, from DPT or DBT.
	DepthMetres float64
	HasDepth    bool
	// DepthBelowKeel is the water depth beneath the lowest point of the hull.
	// It is not the same as DepthMetres: a transducer mounted below the waterline
	// reads shallower than the keel, and the difference is the offset.
	DepthBelowKeel float64
	HasKeelDepth   bool
	// DepthBelowSurface is the water depth below the surface, from DBS. It
	// differs from DepthMetres by the transducer's height above the water.
	DepthBelowSurface float64
	HasSurfaceDepth   bool

	// MagneticVariation is degrees, and its sign follows the E/W field
	// that follows it in the sentence.
	MagneticVariation float64
	HasMagVariation   bool

	// TimeOfDay and Date come from whichever sentence carried them; they
	// are merged so an RMC timestamp is not lost by a later GGA.
	TimeOfDay TOD
	Date      Date

	// Quality is the GGA fix-quality indicator: GPS, DGPS, RTK fixed, and
	// so on. QualityUnknown means no quality field has arrived yet.
	Quality FixQuality

	// NavStatus is the NMEA 4.10 navigational status: safe, caution,
	// unsafe, or not valid. It is stricter than Quality, since a receiver
	// can report a good fix quality and still mark it unsafe to navigate on.
	// Valid already accounts for it, so this is for display and for a
	// program that wants to distinguish caution from outright failure.
	NavStatus    NavStatus
	HasNavStatus bool

	// SatellitesUsed is the count from GSA, SatellitesInView the count
	// from GSV, and Satellites each tracked satellite with its elevation,
	// azimuth, and SNR.
	SatellitesUsed    int
	HasSatellitesUsed bool
	SatellitesInView  int
	HasSatellitesView bool
	// Satellites is every tracked satellite, across all systems. It is the
	// convenience view, rebuilt by SetSatellites; use SatellitesByConstellation
	// when the system matters, because a PRN is only unique within one.
	Satellites []Satellite

	// satelliteGroups holds the same satellites split by system. A multi-GNSS
	// receiver sends a separate GSV cycle per constellation, so a single flat
	// list would be emptied every time the next system's cycle began and a
	// sky view would show only whichever system happened to speak last.
	satelliteGroups []satelliteGroup

	// Dilution of precision. Lower is better; HDOP under 2 is a
	// single-point fix, under 1 is good.
	PDOP, HDOP, VDOP float64
	HasPDOP          bool
	HasHDOP          bool
	HasVDOP          bool

	// HorizontalError and VerticalError are the receiver's own expected
	// error in metres, from a GST sentence. They are a direct measurement
	// and are better than the HorizontalAccuracy estimate derived from HDOP,
	// which is only as good as the 5 m-per-HDOP rule of thumb.
	HorizontalError    float64
	HasHorizontalError bool
	VerticalError      float64
	HasVerticalError   bool

	// SatelliteFault is the PRN a GBS sentence named as the likely source of
	// error, and hasSatelliteFault says whether one was named. The receiver
	// may already have excluded that satellite from the solution, so a fault
	// is recorded rather than treated as invalidating the fix.
	SatelliteFault    int
	HasSatelliteFault bool

	// SatelliteUses holds the satellite numbers in the solution, one entry per
	// GSA sentence that reported them.
	//
	// It is a list rather than a single set because a multi-GNSS receiver
	// sends one GSA per system for each fix, and those have to be merged. A
	// receiver configured for GPS, GLONASS, and Galileo emits three, each
	// naming its own satellites, and overwriting on each one would leave only
	// the last system's satellites marked as used.
	SatelliteUses []SatelliteUse

	// LastSentence is the address of the sentence that last changed this
	// fix, e.g. "GNGGA". LastUpdate is when that happened locally.
	LastSentence string
	LastUpdate   time.Time
	// SentenceCount is how many sentences have contributed since the
	// parser started, which is a cheap way to see the stream is alive.
	SentenceCount uint64
}

// SatelliteUse is one GSA sentence's contribution to the solution: the
// satellites that system had in use, and which sentence said so.
//
// The address and system are kept because the same PRN number means different
// satellites in different constellations, so a bare list of numbers cannot say
// which system a used satellite belongs to.
type SatelliteUse struct {
	// Constellation is the system these satellites belong to, taken from the
	// GSA's system id when it carries one and from its talker otherwise.
	Constellation Constellation
	// System is the NMEA 4.10 system id, or SystemIDNone when absent.
	System SystemID
	// IDs are the satellite numbers in slot order, blanks omitted.
	IDs []int
}

// SetSatellites records the satellites one GSV sentence described for one
// system, accumulating onto that system's existing list.
//
// A GSV cycle is split across up to four sentences and a multi-GNSS receiver
// runs a separate cycle per constellation, so both levels have to accumulate.
// Replacing rather than merging would report a sky of four satellites when the
// receiver can see eleven, and would empty the whole list every time the next
// constellation's cycle started.
//
// replace is true for the first sentence of a cycle, which is what resets the
// system: a satellite that has gone out of view must not linger, and a repeated
// cycle must not grow the list without bound.
func (f *Fix) SetSatellites(c Constellation, known bool, sats []Satellite, replace bool, inView int, hasInView bool) {
	if replace {
		kept := make([]satelliteGroup, 0, len(f.satelliteGroups)+1)
		for _, g := range f.satelliteGroups {
			if g.Constellation != c {
				kept = append(kept, g)
			}
		}
		f.satelliteGroups = append(kept, satelliteGroup{Constellation: c, ConstellationKnown: known})
	}

	idx := -1
	for i := range f.satelliteGroups {
		if f.satelliteGroups[i].Constellation == c {
			idx = i
			break
		}
	}
	if idx < 0 {
		f.satelliteGroups = append(f.satelliteGroups,
			satelliteGroup{Constellation: c, ConstellationKnown: known})
		idx = len(f.satelliteGroups) - 1
	} else if known {
		f.satelliteGroups[idx].ConstellationKnown = true
	}
	if hasInView {
		f.satelliteGroups[idx].InView = inView
		f.satelliteGroups[idx].HasInView = true
	}

	merged := make([]Satellite, 0, len(f.satelliteGroups[idx].Satellites)+len(sats))
	merged = append(merged, f.satelliteGroups[idx].Satellites...)
	for _, sat := range sats {
		// Replace an existing entry with the same number, so a repeated
		// sentence or an overlapping cycle cannot list a satellite twice.
		replaced := false
		for i := range merged {
			if merged[i].ID == sat.ID {
				merged[i] = sat
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, sat)
		}
	}
	f.satelliteGroups[idx].Satellites = merged
	f.rebuildSatellites()
}

// rebuildSatellites regenerates the flat convenience list and the in-view
// count from the per-system groups. It runs on every GSV sentence, over at
// most a few dozen satellites, which is far too little work to be worth
// avoiding.
//
// The in-view count is summed rather than taken from the last sentence, because
// each constellation runs its own cycle and each reports only its own total.
// Overwriting would leave the count describing one constellation while the
// satellite list describes all of them.
func (f *Fix) rebuildSatellites() {
	total, inView, anyInView := 0, 0, false
	for _, g := range f.satelliteGroups {
		total += len(g.Satellites)
		if g.HasInView {
			inView += g.InView
			anyInView = true
		}
	}
	if anyInView {
		f.SatellitesInView, f.HasSatellitesView = inView, true
	}
	if total == 0 {
		f.Satellites = nil
		return
	}
	out := make([]Satellite, 0, total)
	for _, g := range f.satelliteGroups {
		out = append(out, g.Satellites...)
	}
	f.Satellites = out
}

// SatellitesFor returns the tracked satellites of one system, in the order the
// receiver reported them.
func (f Fix) SatellitesFor(c Constellation) []Satellite {
	for _, g := range f.satelliteGroups {
		if g.Constellation == c {
			return g.Satellites
		}
	}
	return nil
}

// SetSatelliteUse records which satellites a GSA sentence reported as being in
// the solution. A decoder calls it; a program reading a Fix does not need to.
//
// Each call replaces the previous list from the same system, which is what
// keeps a multi-GNSS receiver's per-system GSA sentences from overwriting one
// another while still letting the next fix's GSA for that system replace it.
//
// The system is the key rather than the sentence address on purpose. A
// multi-GNSS receiver sends one GSA per system for each fix and they all carry
// the same GN talker, so keying on the address would let each one overwrite the
// last and leave only a single system's satellites in the solution.
func (f *Fix) SetSatelliteUse(constellation Constellation, sys SystemID, ids []int) {
	key := systemKey(constellation, sys)

	kept := make([]SatelliteUse, 0, len(f.SatelliteUses)+1)
	for _, u := range f.SatelliteUses {
		if u.key() != key {
			kept = append(kept, u)
		}
	}
	// A fresh copy, because the GSA's own slice may be reused by the decoder.
	own := append([]int(nil), ids...)
	kept = append(kept, SatelliteUse{
		Constellation: constellation,
		System:        sys,
		IDs:           own,
	})
	f.SatelliteUses = kept

	// The count is derived rather than taken from the sentence, so it cannot
	// disagree with the per-system list. With one system the two are
	// identical; with several, a single GSA's length would undercount.
	total := 0
	for _, u := range f.SatelliteUses {
		total += len(u.IDs)
	}
	f.SatellitesUsed, f.HasSatellitesUsed = total, true
}

// systemKey identifies the reporting system for the purpose of replacing an
// earlier GSA. The system id is preferred because it is explicit; the
// constellation is the fallback for a receiver that does not send one.
func systemKey(c Constellation, sys SystemID) string {
	if sys != SystemIDNone {
		return "sys:" + sys.String()
	}
	return "const:" + c.String()
}

// key is the SatelliteUse method form of systemKey.
func (u SatelliteUse) key() string { return systemKey(u.Constellation, u.System) }

// UsedSatelliteIDs returns every satellite number currently in the solution,
// across all systems, and whether any GSA has been seen.
//
// A PRN is only unique within a constellation, so this is a set of numbers
// rather than a set of identities. Use SatelliteStatus or
// SatellitesByConstellation when the system matters.
func (f Fix) UsedSatelliteIDs() (ids []int, ok bool) {
	for _, u := range f.SatelliteUses {
		ids = append(ids, u.IDs...)
	}
	return ids, len(f.SatelliteUses) > 0
}

// SatelliteStatus pairs a tracked satellite with whether it is in the solution.
type SatelliteStatus struct {
	Satellite
	// Used is true when a GSA sentence listed this satellite's number.
	Used bool
}

// SatelliteStatus joins the GSV and GSA halves of the satellite picture.
//
// The two sentences are independent and arrive in either order, so the join is
// done here rather than stored on either value. The result covers only the
// satellites GSV described; a GSA may name satellites the receiver is using
// without listing them, and those do not appear because there is no elevation,
// azimuth, or signal strength to report for them.
func (f Fix) SatelliteStatus() []SatelliteStatus {
	if len(f.Satellites) == 0 {
		return nil
	}
	// The join is per system, not per number. A PRN is unique only within a
	// constellation: GPS 03, GLONASS 03, and Galileo E03 are three different
	// satellites, and all three are in use at once on a multi-GNSS receiver.
	// Keying on the number alone would mark all of them used because a GSA for
	// one system listed that number.
	used := make(map[satelliteKey]bool, len(f.Satellites))
	for _, u := range f.SatelliteUses {
		for _, id := range u.IDs {
			used[satelliteKey{constellation: u.Constellation, id: id}] = true
		}
	}
	out := make([]SatelliteStatus, 0, len(f.Satellites))
	for _, sat := range f.Satellites {
		key := satelliteKey{constellation: sat.Constellation, id: sat.ID}
		// A GSA that did not name its system, or a satellite whose system the
		// GSV talker did not identify, is matched on the number alone. That is
		// the best available answer and is the same one the pre-NMEA-4.10
		// single-system case gave.
		match := used[key]
		if !match {
			for k := range used {
				if k.id != sat.ID {
					continue
				}
				if k.constellation == ConstellationUnknown || sat.Constellation == ConstellationUnknown {
					match = true
					break
				}
			}
		}
		out = append(out, SatelliteStatus{Satellite: sat, Used: match})
	}
	return out
}

// satelliteKey identifies a satellite within its own system, which is the
// narrowest identity a GSV or GSA sentence actually provides.
type satelliteKey struct {
	constellation Constellation
	id            int
}

// ConstellationStatus is the tracked and used count for one system.
type ConstellationStatus struct {
	Constellation Constellation
	// Tracked is how many satellites GSV described for this system.
	Tracked int
	// Used is how many of them a GSA listed in the solution.
	Used int
	// ConstellationKnown is false when the GSV talker was not recognised, in
	// which case the satellites are counted under ConstellationUnknown.
	ConstellationKnown bool
}

// SatellitesByConstellation summarises the sky per system, which is the view
// that answers "which constellations am I actually using".
//
// The entries are ordered by Constellation, so the result is stable and can be
// printed directly. A GNGSV contributes a single ConstellationMixed entry,
// because that sentence reports a fused set without saying which system each
// satellite belongs to.
func (f Fix) SatellitesByConstellation() []ConstellationStatus {
	byConst := make(map[Constellation]*ConstellationStatus)
	status := f.SatelliteStatus()

	// The per-system groups are the authority, so a system that has satellites
	// tracked but none of them in the GSA solution still appears.
	for _, g := range f.satelliteGroups {
		cs, ok := byConst[g.Constellation]
		if !ok {
			cs = &ConstellationStatus{Constellation: g.Constellation}
			byConst[g.Constellation] = cs
		}
		if g.ConstellationKnown {
			cs.ConstellationKnown = true
		}
	}

	// A system named only by a GSA, with no GSV coverage, still belongs in the
	// table: it is in use, which is the question the table answers.
	for _, u := range f.SatelliteUses {
		if _, ok := byConst[u.Constellation]; !ok {
			byConst[u.Constellation] = &ConstellationStatus{Constellation: u.Constellation}
		}
	}

	for _, s := range status {
		cs, ok := byConst[s.Constellation]
		if !ok {
			cs = &ConstellationStatus{
				Constellation:      s.Constellation,
				ConstellationKnown: s.ConstellationKnown,
			}
			byConst[s.Constellation] = cs
		}
		if s.ConstellationKnown {
			cs.ConstellationKnown = true
		}
		cs.Tracked++
		if s.Used {
			cs.Used++
		}
	}

	out := make([]ConstellationStatus, 0, len(byConst))
	for _, cs := range byConst {
		// A system named only by a GSA, with no GSV coverage and no satellites
		// listed, is noise in a table meant to describe the sky.
		if cs.Tracked == 0 && cs.Used == 0 {
			continue
		}
		out = append(out, *cs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Constellation < out[j].Constellation })
	return out
}

// satelliteGroup is one system's tracked satellites, accumulated across that
// system's GSV cycle.
type satelliteGroup struct {
	Constellation      Constellation
	ConstellationKnown bool
	Satellites         []Satellite
	// InView is the count the receiver reported for this system, which is a
	// cycle total rather than a sentence count.
	InView int
	// HasInView is false until a GSV for this system has arrived.
	HasInView bool
}

// Satellite is one tracked space vehicle from a GSV sentence.
//
// A GSV cycle is split across several sentences and does not necessarily
// describe every satellite the receiver can hear, so "in view" here means
// what the receiver reported, not an exhaustive sky.
type Satellite struct {
	// ID is the satellite number, which is only unique within a
	// constellation: GPS 12 and GLONASS 12 are different objects.
	ID        int
	Elevation float64
	Azimuth   float64
	SNR       float64
	HasSNR    bool

	// Constellation is the system this satellite belongs to.
	//
	// In multi-GNSS operation NMEA requires the GSV talker to be specific to
	// the system being reported, so a GAGSV really is Galileo and a GBGSV
	// really is BeiDou, even while every other sentence uses the GN talker.
	// A GNGSV means the receiver is reporting a fused set it does not
	// attribute, and Constellation is then ConstellationMixed.
	Constellation Constellation
	// ConstellationKnown is false only when the talker was not one this
	// library recognises, which is a different situation from a fused set:
	// there the satellites are known to come from somewhere, just not from a
	// system that can be named. It is true for ConstellationMixed, which is
	// a fact about the data rather than a gap in it.
	ConstellationKnown bool

	// Signal and HasSignal carry the NMEA 4.10 signal digit, which identifies
	// the band once it is resolved against Constellation: L1 C/A, L2C-M, E5a,
	// B1I, and so on. Use SignalName to get that name.
	//
	// Older receivers send no signal field at all, so HasSignal is false for
	// every pre-4.10 stream. A receiver that does send one may send 0, meaning
	// "all signals", which HasSignal reports as true and SignalName declines
	// to name.
	Signal    SignalDigit
	HasSignal bool
}

// There is deliberately no Used field on Satellite. Whether a satellite is in
// the solution comes from GSA, which is a different sentence from the GSV that
// describes the satellite, and the two arrive in either order. A flag stored on
// the GSV value would be wrong for every sentence between the two updates, so
// the join is done on demand by Fix.SatelliteStatus instead.

// SignalName returns the band name for this satellite, resolved against its own
// constellation, and whether one is available.
//
// The constellation matters: signal id 1 is L1 C/A for GPS but E1C for Galileo
// and B1I for BeiDou, so a name resolved without it would be wrong for two
// systems out of three. The second result is false when the receiver sent no
// signal id, which is every pre-4.10 receiver and some modern ones.
func (s Satellite) SignalName() (string, bool) {
	if !s.HasSignal {
		return "", false
	}
	sys := SystemIDOf(s.Constellation)
	if sys == SystemIDNone {
		return "", false
	}
	// Band reports false for digit 0, which means "all signals" rather than a
	// specific band, and for a digit the system does not define. Both are
	// honest refusals rather than failures. The bare digit is still available
	// through Signal.String for a caller that wants to show it.
	band, ok := s.Signal.Band(sys)
	if !ok || band == SignalIDAll {
		return "", false
	}
	return band.String(), true
}

// String renders one satellite for a log line or a sky plot, for example
// "GPS 12 L1 C/A 47d 121d 42dB".
func (s Satellite) String() string {
	var b strings.Builder

	switch {
	case s.ConstellationKnown && s.Constellation.IsMixed():
		b.WriteString("MIX ")
	case s.ConstellationKnown:
		fmt.Fprintf(&b, "%-4s", abbreviate(s.Constellation))
	default:
		b.WriteString("?    ")
	}
	fmt.Fprintf(&b, "%3d", s.ID)

	if name, ok := s.SignalName(); ok {
		b.WriteString(" ")
		b.WriteString(name)
	} else if s.HasSignal && s.Signal == 0 {
		b.WriteString(" all")
	}
	fmt.Fprintf(&b, "  %3.0f° %3.0f°", s.Elevation, s.Azimuth)
	if s.HasSNR {
		fmt.Fprintf(&b, " %2.0fdB", s.SNR)
	}
	return b.String()
}

// abbreviate shortens a constellation to a fixed-width tag for a column, so a
// sky view lines up.
func abbreviate(c Constellation) string {
	switch c {
	case ConstellationGPS:
		return "GPS"
	case ConstellationGLONASS:
		return "GLO"
	case ConstellationGalileo:
		return "GAL"
	case ConstellationBeiDou:
		return "BDS"
	case ConstellationQZSS:
		return "QZS"
	case ConstellationNavIC:
		return "NAV"
	case ConstellationSBAS:
		return "SBA"
	case ConstellationMixed:
		return "MIX"
	default:
		return "?"
	}
}

// ConstellationAbbrev is the short form of a constellation name, for a table
// column or a plot legend.
func ConstellationAbbrev(c Constellation) string { return abbreviate(c) }

// Reset clears the fix while keeping the counters, so a stream restart or
// a mode change does not leave stale coordinates visible.
func (f *Fix) Reset() {
	sentences := f.SentenceCount
	*f = Fix{SentenceCount: sentences}
}

// applyAt merges a value that implements FixContributor, stamping the
// update time and the address of the sentence that caused the change.
//
// A Fix normally changes several times per second, so the parser does not
// try to diff old against new state; the values are simply overwritten and
// the presence flags make partial updates correct.
func (f *Fix) applyAt(c FixContributor, now time.Time, address string) {
	c.ApplyFix(f)
	f.SentenceCount++
	f.LastUpdate = now
	f.LastSentence = address
}

// SetPosition validates and stores a coordinate pair, setting HasPosition
// only if both halves are valid. It is how every sentence decoder folds a
// position into the fix, so the presence rules live in one place.
func (f *Fix) SetPosition(lat, lon Coordinate) error {
	if !lat.Valid || !lon.Valid {
		return fault.ErrNotValid
	}
	latDeg, err := lat.Decimal()
	if err != nil {
		return err
	}
	lonDeg, err := lon.Decimal()
	if err != nil {
		return err
	}
	f.Latitude, f.Longitude = latDeg, lonDeg
	f.HasPosition = true
	return nil
}

// SetPositionDecimal stores a position already expressed in signed decimal
// degrees, range-checking it. It is what a sentence decoder that has already
// converted its coordinates calls, so the range check happens in exactly one
// place.
func (f *Fix) SetPositionDecimal(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return fault.ErrFieldValue
	}
	if lat < -90 || lat > 90 {
		return fault.ErrFieldRange
	}
	if lon < -180 || lon > 180 {
		return fault.ErrFieldRange
	}
	f.Latitude, f.Longitude = lat, lon
	f.HasPosition = true
	return nil
}

// SetSpeedKnots stores ground speed and its unit conversions, so the three
// representations can never drift apart.
func (f *Fix) SetSpeedKnots(knots float64) {
	f.SpeedKnots = knots
	f.SpeedKmh = knots * 1.852
	f.SpeedMps = knots * 1852.0 / 3600.0
	f.HasSpeed = true
}

// DecimalPosition returns the position in signed decimal degrees, and false
// when the receiver has not supplied one.
//
// This is the call almost every program wants. Positive latitude is north and
// positive longitude is east.
func (f Fix) DecimalPosition() (lat, lon float64, ok bool) {
	return f.Latitude, f.Longitude, f.HasPosition
}

// Timestamp returns the merged date and time of day as a single time.Time.
// It reports false when the receiver has not sent enough information, which
// is normal: a GPS module that has only ever emitted GGA has no date.
func (f Fix) Timestamp() (time.Time, bool) { return f.Date.Combine(f.TimeOfDay) }

// HorizontalAccuracy approximates the ground error in metres from HDOP
// using 1 HDOP = 5 m, the usual rule of thumb for a single-frequency
// receiver.
//
// A measured HorizontalError from a GST sentence is preferred when present,
// since it comes from the receiver's own residuals. This method is the
// fallback for receivers that report no GST.
func (f Fix) HorizontalAccuracy() (metres float64, ok bool) {
	if f.HasHorizontalError {
		return f.HorizontalError, true
	}
	if !f.HasHDOP {
		return 0, false
	}
	return f.HDOP * 5.0, true
}

// String summarises the fix for logging, e.g.
// "gps fix 51.4779, -0.0015 +/-3.1m 8/11 sats 12:00:03".
func (f Fix) String() string {
	if !f.HasPosition {
		return "no position yet"
	}
	s := f.Quality.String() + " fix " +
		formatCoord(f.Latitude, AxisLatitude) + ", " +
		formatCoord(f.Longitude, AxisLongitude)
	if acc, ok := f.HorizontalAccuracy(); ok {
		s += " +/-" + trimFloat(acc) + "m"
	}
	if f.HasSatellitesUsed || f.HasSatellitesView {
		s += " " + strconv.Itoa(f.SatellitesUsed) + "/" + strconv.Itoa(f.SatellitesInView) + " sats"
	}
	if f.HasSpeed {
		s += " " + trimFloat(f.SpeedKnots) + "kn"
	}
	if ts, ok := f.Timestamp(); ok {
		s += " " + ts.Format("15:04:05")
	}
	return s
}

func formatCoord(deg float64, axis Axis) string {
	if axis == AxisLongitude {
		return trimFloat(deg) + " lon"
	}
	return trimFloat(deg) + " lat"
}

// trimFloat formats a float without an exponent and without trailing zeros,
// which is what a log line wants.
func trimFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
