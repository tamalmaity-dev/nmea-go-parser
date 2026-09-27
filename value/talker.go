package value

import (
	"strconv"
	"strings"
)

// Constellation is a satellite system: GPS, GLONASS, Galileo, BeiDou, QZSS,
// NavIC, or SBAS.
type Constellation int

const (
	// ConstellationUnknown is an unrecognised or absent system.
	ConstellationUnknown Constellation = iota
	ConstellationGPS
	ConstellationGLONASS
	ConstellationGalileo
	ConstellationBeiDou
	ConstellationQZSS
	ConstellationNavIC
	// ConstellationSBAS is a satellite-based augmentation system: WAAS, EGNOS,
	// MSAS, GAGAN, and SDCM. It is tracked like any other constellation, but
	// unlike the others it has no talker prefix and no system id of its own:
	// SBAS satellites arrive under the GPS talker with system id 1, so the
	// only way to tell one from a GPS satellite is its PRN, which is in the
	// range 120 to 158. Use IsSBASPRN for that.
	ConstellationSBAS
	// ConstellationMixed is the "GN" talker, meaning a fused solution from
	// several systems at once.
	ConstellationMixed
)

// IsSBASPRN reports whether a satellite number belongs to an SBAS payload
// rather than to a GNSS constellation.
//
// This is the only way to tell the two apart on the wire. SBAS has no talker
// of its own and shares NMEA system id 1 with GPS, so PRN 120 is the sole
// discriminator: the range below it is GPS, and 120 to 158 is SBAS. Extended
// satellite numbering pushes some real SBAS PRNs as high as 158.
func IsSBASPRN(prn int) bool { return prn >= 120 && prn <= 158 }

func (c Constellation) String() string {
	switch c {
	case ConstellationGPS:
		return "GPS"
	case ConstellationGLONASS:
		return "GLONASS"
	case ConstellationGalileo:
		return "Galileo"
	case ConstellationBeiDou:
		return "BeiDou"
	case ConstellationQZSS:
		return "QZSS"
	case ConstellationNavIC:
		return "NavIC"
	case ConstellationSBAS:
		return "SBAS"
	case ConstellationMixed:
		return "Mixed"
	default:
		return "Unknown"
	}
}

// IsMixed reports whether the constellation is a fused multi-system solution
// rather than one system, which is what the "GN" talker means.
func (c Constellation) IsMixed() bool { return c == ConstellationMixed }

// SystemID is the NMEA 4.10 numeric system identifier, carried in the GSA,
// GRS, and GBS sentences. It duplicates information the talker already
// implies, but it is the only field that identifies the system when a
// receiver uses "GN" for everything.
type SystemID int

const (
	// SystemIDNone is an absent or unrecognised system id.
	SystemIDNone SystemID = iota
	SystemIDGPS
	SystemIDGLONASS
	SystemIDGalileo
	SystemIDBeiDou
	SystemIDQZSS
	SystemIDNavIC
)

func (s SystemID) String() string {
	switch s {
	case SystemIDGPS:
		return "GPS"
	case SystemIDGLONASS:
		return "GLONASS"
	case SystemIDGalileo:
		return "Galileo"
	case SystemIDBeiDou:
		return "BeiDou"
	case SystemIDQZSS:
		return "QZSS"
	case SystemIDNavIC:
		return "NavIC"
	default:
		return "none"
	}
}

// Constellation maps a system id to its constellation.
func (s SystemID) Constellation() Constellation {
	switch s {
	case SystemIDGPS:
		return ConstellationGPS
	case SystemIDGLONASS:
		return ConstellationGLONASS
	case SystemIDGalileo:
		return ConstellationGalileo
	case SystemIDBeiDou:
		return ConstellationBeiDou
	case SystemIDQZSS:
		return ConstellationQZSS
	case SystemIDNavIC:
		return ConstellationNavIC
	default:
		return ConstellationUnknown
	}
}

// SystemIDOf is the reverse of SystemID.Constellation, which is what lets a
// sentence that names its system by talker, such as GSV, still resolve a
// signal id into a band name.
//
// SBAS deliberately returns SystemIDGPS, because that is the id it actually
// transmits. Reporting it as a system of its own would be a fiction the wire
// does not support.
func SystemIDOf(c Constellation) SystemID {
	switch c {
	case ConstellationGPS, ConstellationSBAS:
		return SystemIDGPS
	case ConstellationGLONASS:
		return SystemIDGLONASS
	case ConstellationGalileo:
		return SystemIDGalileo
	case ConstellationBeiDou:
		return SystemIDBeiDou
	case ConstellationQZSS:
		return SystemIDQZSS
	case ConstellationNavIC:
		return SystemIDNavIC
	default:
		return SystemIDNone
	}
}

// ParseSystemID converts the numeric field. Zero is deliberately not a
// constellation: Trimble receivers emit 0 for QZSS, which would otherwise be
// silently mislabelled as "no system".
func ParseSystemID(s string) (SystemID, bool) {
	switch strings.TrimSpace(s) {
	case "1":
		return SystemIDGPS, true
	case "2":
		return SystemIDGLONASS, true
	case "3":
		return SystemIDGalileo, true
	case "4":
		return SystemIDBeiDou, true
	case "5":
		return SystemIDQZSS, true
	case "6":
		return SystemIDNavIC, true
	default:
		return SystemIDNone, false
	}
}

// SignalID names a band, such as L1 C/A or E5a. It is a name, not a number:
// the same digit on the wire means different bands for different systems, so a
// band cannot also be the digit that carried it. SignalDigit is the digit;
// Band resolves one to the other.
//
// The numeric values here are deliberately not the wire digits. Nothing
// compares a SignalID against a raw field, and giving these the wire values
// would invite exactly that.
type SignalID int

const (
	SignalIDUnknown SignalID = iota
	SignalIDAll              // 0, meaning "all signals" rather than one specific band
	SignalIDL1C
	SignalIDL1P
	SignalIDL1M
	SignalIDL2P
	SignalIDL2CM
	SignalIDL2CL
	SignalIDL5I
	SignalIDL5Q
	SignalIDE1a
	SignalIDE1b
	SignalIDE5a
	SignalIDE5b
	SignalIDE6a
	SignalIDE6bc
	SignalIDB1I
	SignalIDB1Q
	SignalIDB1C
	SignalIDB1A
	SignalIDB2a
	SignalIDB2b
	SignalIDB2I
	SignalIDB2Q
	SignalIDB3I
	SignalIDB3Q
	SignalIDB3A
	// GLONASS broadcasts only two civil bands, named L1 and L2 with no code
	// suffix, which is why they are separate from the GPS L1C and L2C names
	// even though the frequency is the same.
	SignalIDL1
	SignalIDL2
	// SignalIDL1S is the QZSS-only L1 signal on digit 4, and SignalIDL5A is
	// NavIC's sole signal, which the standard spells without the I or Q that
	// distinguish the GPS bands at the same frequency.
	SignalIDL1S
	SignalIDL5A
)

func (s SignalID) String() string {
	switch s {
	case SignalIDAll:
		return "all signals"
	case SignalIDL1C:
		return "L1 C/A"
	case SignalIDL1P:
		return "L1 P(Y)"
	case SignalIDL1M:
		return "L1 M"
	case SignalIDL2P:
		return "L2 P(Y)"
	case SignalIDL2CM:
		return "L2C-M"
	case SignalIDL2CL:
		return "L2C-L"
	case SignalIDL5I:
		return "L5-I"
	case SignalIDL5Q:
		return "L5-Q"
	case SignalIDE1a:
		return "E1a"
	case SignalIDE1b:
		return "E1b"
	case SignalIDE5a:
		return "E5a"
	case SignalIDE5b:
		return "E5b"
	case SignalIDE6a:
		return "E6-A"
	case SignalIDE6bc:
		return "E6-BC"
	case SignalIDB1I:
		return "B1I"
	case SignalIDB1Q:
		return "B1Q"
	case SignalIDB1C:
		return "B1C"
	case SignalIDB1A:
		return "B1A"
	case SignalIDB2a:
		return "B2-a"
	case SignalIDB2b:
		return "B2-b"
	case SignalIDB2I:
		return "B2I"
	case SignalIDB2Q:
		return "B2Q"
	case SignalIDB3I:
		return "B3I"
	case SignalIDB3Q:
		return "B3Q"
	case SignalIDB3A:
		return "B3A"
	case SignalIDL1:
		return "L1"
	case SignalIDL2:
		return "L2"
	case SignalIDL1S:
		return "L1S"
	case SignalIDL5A:
		return "L5-A"
	default:
		return "unknown signal"
	}
}

// ParseSignalID reads the signal field, which is a single hexadecimal digit in
// every version that carries it, and returns it as a SignalDigit.
//
// The result is a digit rather than a band because the constellations overlap
// on digit values: 1 is L1 C/A for GPS but E1C for Galileo and B1I for BeiDou.
// Call SignalDigit.Band with the satellite's system to get a name.
func ParseSignalID(s string) (SignalDigit, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DigitNone, false
	}
	v, err := parseSmallInt(s)
	if err != nil {
		return DigitNone, false
	}
	return SignalDigit(v), true
}

// SignalDigit is the NMEA 4.10 signal identifier exactly as it arrives: a
// single hexadecimal digit.
//
// It is kept apart from SignalID because the digit means different bands for
// different systems. Digit 1 is L1 C/A for GPS, E1C for Galileo, B1I for
// BeiDou, L1 for GLONASS, and L5-A for NavIC, so a digit on its own does not
// name a band. Band resolves it against the system it arrived with.
//
// A receiver sends 0 to mean "all signals", which is a real answer rather than
// a missing one, and it is Band's job to report that as such.
type SignalDigit int

// DigitNone is the absence of a signal digit, which is what every pre-4.10
// receiver produces because it sends no such field.
const DigitNone SignalDigit = -1

// Band resolves the digit to a band name for the given system, and reports
// false when the system does not define that digit or the system is unknown.
//
// The second result is the honest failure: a digit with no defined meaning is
// not the same as no digit, and reporting a name anyway would be a guess.
func (d SignalDigit) Band(sys SystemID) (SignalID, bool) {
	if d == DigitNone {
		return SignalIDUnknown, false
	}
	table, ok := signalTables[sys]
	if !ok {
		return SignalIDUnknown, false
	}
	band, ok := table[int(d)]
	return band, ok
}

// String renders the raw digit, which is all that can be said about it without
// a system. An absent digit renders as a question mark rather than as 0,
// because 0 on the wire means "all signals".
func (d SignalDigit) String() string {
	if d == DigitNone {
		return "?"
	}
	return strconv.FormatInt(int64(d), 10)
}

// signalTables maps a system to its signal digit assignments. A digit a system
// does not define is absent on purpose, so Band reports false for it instead of
// inventing a band.
var signalTables = map[SystemID]map[int]SignalID{
	SystemIDGPS:     signalNamesGPS,
	SystemIDGalileo: signalNamesGalileo,
	SystemIDBeiDou:  signalNamesBeiDou,
	SystemIDQZSS:    signalNamesQZSS,
	SystemIDGLONASS: signalNamesGLONASS,
	SystemIDNavIC:   signalNamesNavIC,
}

// The signal id tables, one per system.
//
// A signal id is a single hex digit whose meaning depends on the system it is
// paired with, which is the detail that makes this fiddly: digit 1 is L1 C/A
// for GPS, E1C for Galileo, B1I for BeiDou, and L1 for GLONASS. Reading a
// Galileo satellite's signal id with the GPS table reports E1C as "L1 C/A",
// which is plausible enough to be believed and wrong.
//
// The digits below are the NMEA 4.11 assignments.
//
// SBAS has no table of its own: it transmits system id 1 and its only signal is
// L1 C/A, so it resolves through the GPS table.
var (
	// signalNamesGPS covers GPS, the commonest pairing. Digits 2, 3, and 4 are
	// the legacy single-frequency assignments; the 4.10 and later tables do
	// not define them, but receivers in the field still emit them, so they are
	// named rather than dropped.
	signalNamesGPS = map[int]SignalID{
		0: SignalIDAll,
		1: SignalIDL1C, 2: SignalIDL1P, 3: SignalIDL1M, 4: SignalIDL2P,
		5: SignalIDL2CM, 6: SignalIDL2CL, 7: SignalIDL5I, 8: SignalIDL5Q,
	}

	// signalNamesGalileo reuses several digits for entirely different bands:
	// 2 is E5b, not E1b, and 4 is E6-A, not E5b. Digit 6 is not defined for
	// Galileo and is deliberately missing.
	signalNamesGalileo = map[int]SignalID{
		0: SignalIDAll,
		1: SignalIDE1a, 2: SignalIDE5b, 3: SignalIDE5a,
		4: SignalIDE6a, 5: SignalIDE6bc, 7: SignalIDE1a,
	}

	// signalNamesBeiDou names the B1, B2, and B3 bands. The pilot and data
	// components of B1 share a digit, and the standard does not separate
	// them, so both map to the same name.
	signalNamesBeiDou = map[int]SignalID{
		0: SignalIDAll,
		1: SignalIDB1I, 2: SignalIDB2I,
		3: SignalIDB1C, 4: SignalIDB1A, 5: SignalIDB2a, 8: SignalIDB3I,
	}

	// signalNamesQZSS is GPS's set plus the L1S signal, on digit 4.
	signalNamesQZSS = map[int]SignalID{
		0: SignalIDAll,
		1: SignalIDL1C, 4: SignalIDL1S,
		5: SignalIDL2CM, 6: SignalIDL2CL, 7: SignalIDL5I, 8: SignalIDL5Q,
	}

	// signalNamesGLONASS has only two bands, and its L2 sits on digit 3
	// rather than 2, so it shares nothing with the GPS table beyond digit 1.
	signalNamesGLONASS = map[int]SignalID{
		0: SignalIDAll,
		1: SignalIDL1, 3: SignalIDL2,
	}

	// signalNamesNavIC is L5-A only, which is its sole civil signal.
	signalNamesNavIC = map[int]SignalID{
		0: SignalIDAll,
		1: SignalIDL5A,
	}
)

// talkerTable maps the two-character talker prefix to a constellation.
//
// Several prefixes are accepted for the same system because receivers in the
// field disagree: BeiDou appears as both BD and GB, and QZSS as both GQ and
// QZ. The standard does not date either to a specific revision, so a parser
// that rejected the "wrong" one would fail on real hardware.
var talkerTable = map[string]Constellation{
	"GP": ConstellationGPS,
	"GL": ConstellationGLONASS,
	"GA": ConstellationGalileo,
	"GB": ConstellationBeiDou,
	"BD": ConstellationBeiDou,
	"GQ": ConstellationQZSS,
	"QZ": ConstellationQZSS,
	"GI": ConstellationNavIC,
	"IR": ConstellationNavIC,
	"GN": ConstellationMixed,
	"PQ": ConstellationQZSS, // Quectel's non-standard QZSS prefix
}

// TalkerConstellation identifies which satellite system a sentence came
// from. An unrecognised talker yields ConstellationUnknown, which is
// different from an error: receivers emit talkers outside the GNSS set all
// the time, for AIS, sonar, and depth sounders.
func TalkerConstellation(talker string) Constellation {
	if c, ok := talkerTable[strings.ToUpper(talker)]; ok {
		return c
	}
	return ConstellationUnknown
}

// TalkerSystemID is TalkerConstellation expressed as the NMEA 4.10 system
// id, which is what a GSA, GRS, or GBS field compares against.
func TalkerSystemID(talker string) SystemID {
	switch TalkerConstellation(talker) {
	case ConstellationGPS:
		return SystemIDGPS
	case ConstellationGLONASS:
		return SystemIDGLONASS
	case ConstellationGalileo:
		return SystemIDGalileo
	case ConstellationBeiDou:
		return SystemIDBeiDou
	case ConstellationQZSS:
		return SystemIDQZSS
	case ConstellationNavIC:
		return SystemIDNavIC
	default:
		return SystemIDNone
	}
}

// KnownTalkers returns every talker prefix this package recognises, which is
// useful for a CLI listing what constellations it can tell apart.
func KnownTalkers() []string {
	out := make([]string, 0, len(talkerTable))
	for t := range talkerTable {
		out = append(out, t)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	// A tiny insertion sort keeps this dependency-free and the list is
	// always under twenty entries.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
