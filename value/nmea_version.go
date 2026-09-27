package value

import (
	"fmt"
	"strings"
)

// Version is a revision of the NMEA 0183 standard.
//
// The version number is a major.minor pair, not a semver: 4.30 is later than
// 4.11, and reading "30" as a minor of 4 would put 4.11 above it. The
// ordering therefore has to be explicit.
type Version struct {
	Major int
	Minor int
}

// The published revisions. V4_30 is the current standard; everything before
// it is still routinely emitted by receivers in the field.
var (
	V2_00 = Version{2, 0}
	V2_01 = Version{2, 1}
	V2_10 = Version{2, 10}
	V2_20 = Version{2, 20}
	V2_30 = Version{2, 30}
	V3_00 = Version{3, 0}
	V3_01 = Version{3, 1}
	V4_00 = Version{4, 0}
	V4_10 = Version{4, 10}
	V4_11 = Version{4, 11}
	V4_30 = Version{4, 30}
)

// LatestVersion is the current published revision, NMEA 0183 v4.30 from
// December 2023, which replaced v4.11. It corresponds to IEC 61162-1:2024.
var LatestVersion = V4_30

func (v Version) String() string { return fmt.Sprintf("%d.%02d", v.Major, v.Minor) }

// AtLeast reports whether v is the same as or later than other.
//
// The comparison is a plain major-then-minor integer compare, which is
// correct because the minor component has always increased with the major:
// 4.30 really is later than 4.11.
func (v Version) AtLeast(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	return v.Minor >= other.Minor
}

// Before is AtLeast inverted, for the same reason.
func (v Version) Before(other Version) bool { return !v.AtLeast(other) }

// Feature is a capability a receiver may or may not implement, as reported
// by a VER sentence or inferred from the shape of the sentences arriving.
type Feature int

const (
	// FeatureSystemID is the GNSS system identifier in GSA, GRS, and GBS.
	// Added in 4.10.
	FeatureSystemID Feature = iota
	// FeatureSignalID is the signal identifier in GSV, GSA, and GBS.
	// Added in 4.10.
	FeatureSignalID
	// FeatureNavStatus is the navigational status in RMC, added in 4.10.
	FeatureNavStatus
	// FeatureFAA is the FAA mode indicator, added in 2.3.
	FeatureFAA
	// FeatureGroundSpeed is the over-ground half of VLW, added in 3.0.
	FeatureGroundSpeed
	// FeatureRangeScale is the maximum depth range scale in DPT, added in 3.0.
	FeatureRangeScale
)

// FeatureName returns the short name of a feature, for diagnostics.
func FeatureName(f Feature) string {
	switch f {
	case FeatureSystemID:
		return "system-id"
	case FeatureSignalID:
		return "signal-id"
	case FeatureNavStatus:
		return "nav-status"
	case FeatureFAA:
		return "faa-mode"
	case FeatureGroundSpeed:
		return "ground-speed"
	case FeatureRangeScale:
		return "range-scale"
	default:
		return "unknown"
	}
}

// FeatureIntroduced names the revision that added a feature.
func FeatureIntroduced(f Feature) Version {
	switch f {
	case FeatureFAA:
		return V2_30
	case FeatureGroundSpeed, FeatureRangeScale:
		return V3_00
	case FeatureSystemID, FeatureSignalID, FeatureNavStatus:
		return V4_10
	default:
		return Version{}
	}
}

// VersionInfo is what a receiver says about itself, gathered from a VER
// sentence and from the shape of the traffic.
type VersionInfo struct {
	// Sentences lists the formatters the receiver claims to emit.
	Sentences []string

	// Talker is the receiver's own talker ID, e.g. "GP".
	Talker string
	// SystemID is the constellation the receiver identifies itself as, when
	// the VER sentence gave a NMEA 4.10 style system id.
	SystemID int
	// HasSystemID distinguishes a reported zero from an absent field.
	HasSystemID bool

	// Manufacturer, Product, and Version come from a VER sentence, e.g.
	// "$GPVER,SiRF,GSiRF03,1000,1.00" or "$PMLRF,..."
	Manufacturer string
	Product      string
	Software     string

	// ProtocolVersion is the revision the receiver claims, when it reported
	// one. Most receivers do not, which is why InferVersion exists.
	ProtocolVersion Version
	// HasProtocolVersion distinguishes an unreported version from a
	// reported one.
	HasProtocolVersion bool
}

// String summarises the receiver for a log line.
func (v VersionInfo) String() string {
	parts := make([]string, 0, 4)
	if v.Manufacturer != "" {
		parts = append(parts, v.Manufacturer)
	}
	if v.Product != "" {
		parts = append(parts, v.Product)
	}
	if v.Software != "" {
		parts = append(parts, "sw "+v.Software)
	}
	if v.HasProtocolVersion {
		parts = append(parts, "NMEA "+v.ProtocolVersion.String())
	} else {
		parts = append(parts, "NMEA version unreported")
	}
	return strings.Join(parts, " ")
}

// Apply merges a decoded VER sentence into the info.
func (v *VersionInfo) Apply(ver VER) {
	if ver.Manufacturer != "" {
		v.Manufacturer = ver.Manufacturer
	}
	if ver.Product != "" {
		v.Product = ver.Product
	}
	if ver.Software != "" {
		v.Software = ver.Software
	}
	if ver.Talker != "" {
		v.Talker = ver.Talker
	}
	if ver.Sentences != nil {
		v.Sentences = ver.Sentences
	}
	if ver.ProtocolVersion.Major > 0 {
		v.ProtocolVersion = ver.ProtocolVersion
		v.HasProtocolVersion = true
	}
}

// VER is the Version and Model Identification sentence. It is the only way a
// receiver describes itself, and the only direct statement of which revision
// of the standard it speaks.
//
//	$GPVER,SiRF,GSiRF03,1000,1.00*5C
//	$GPRVER,SiRF03,3.2,1000*3E
//	$PMTKVER,3.00,0003*43
//
// Field layout:
//
//	0 manufacturer
//	1 product
//	2 software version
//	3 receiver version
//
// Field 3 is where a receiver that knows its standard revision puts it, so it
// is also offered as ProtocolVersion. Most receivers leave it as a build
// number, which is why nothing should rely on it: HasProtocolVersion is only
// set when the value genuinely looks like an NMEA revision.
//
// The type lives here rather than in the sentences package so that
// VersionInfo can consume it without the two packages depending on each
// other. The decoder is in the sentences package.
type VER struct {
	Manufacturer string
	Product      string
	Software     string
	// Receiver is the fourth field, commonly a build or model number.
	Receiver string
	// ProtocolVersion is field 3 read as an NMEA revision, set only when the
	// value really looks like one.
	ProtocolVersion Version
	// HasProtocolVersion distinguishes a build number from a version.
	HasProtocolVersion bool
	// Sentences is populated for the $PUBX,00 form of VER, which lists the
	// supported formatters instead of product details.
	Sentences []string
	// Talker is the talker the sentence arrived with, kept because a VER
	// often carries the only reliable talker a receiver ever sends.
	Talker string
}

// Feature sets, preallocated. A decoded sentence reports what it implies on
// every sentence that arrives, so returning a fresh slice literal from
// NMEASupports allocated once per sentence for no benefit. Callers must treat
// the returned slices as read-only.
var (
	featureNone         []Feature
	featureFAA          = []Feature{FeatureFAA}
	featureSystemAndSig = []Feature{FeatureSystemID, FeatureSignalID}
	featureSignalOnly   = []Feature{FeatureSignalID}
	featureNavAndFAA    = []Feature{FeatureNavStatus, FeatureFAA}
	featureNavOnly      = []Feature{FeatureNavStatus}
	featureGroundSpeed  = []Feature{FeatureGroundSpeed}
	featureRangeScale   = []Feature{FeatureRangeScale}
)

// FeatureOnly returns a read-only slice naming just f, without allocating.
func FeatureOnly(f Feature) []Feature {
	switch f {
	case FeatureSystemID:
		return []Feature{FeatureSystemID}
	case FeatureSignalID:
		return featureSignalOnly
	case FeatureNavStatus:
		return featureNavOnly
	case FeatureFAA:
		return featureFAA
	case FeatureGroundSpeed:
		return featureGroundSpeed
	case FeatureRangeScale:
		return featureRangeScale
	default:
		return featureNone
	}
}

// FeaturesSystemAndSignal returns the system-id and signal-id pair, which
// several sentences imply together.
func FeaturesSystemAndSignal() []Feature { return featureSystemAndSig }

// FeaturesNavAndFAA returns the navigational-status and FAA-mode pair, which
// RMC implies together.
func FeaturesNavAndFAA() []Feature { return featureNavAndFAA }

// NMEAVersion implements VersionReporter, letting a VER sentence feed the
// version detector directly.
func (v VER) NMEAVersion() (Version, bool) { return v.ProtocolVersion, v.HasProtocolVersion }

// NMEASupports implements FeatureEvidence. A VER that names its standard
// revision also implies the FAA mode field, which has been present since
// NMEA 2.3.
func (v VER) NMEASupports() []Feature {
	if !v.HasProtocolVersion {
		return featureNone
	}
	return featureFAA
}

// AsVER extracts a VER from a decoded value, accepting either a value or a
// pointer. A custom decoder is free to return either, and a version-aware
// parser that silently ignored half of them would report no receiver
// information at all with no indication why.
func AsVER(value any) (VER, bool) {
	switch v := value.(type) {
	case VER:
		return v, true
	case *VER:
		if v != nil {
			return *v, true
		}
	}
	return VER{}, false
}

// ParseVersion reads a standard revision such as "4.11" or "3.01", and
// reports false for anything else.
//
// It is deliberately strict in two ways, both of which have bitten real
// receivers:
//
//   - The minor component must be exactly two digits. Every published
//     revision uses two, including 4.30, and accepting "3.2" would leave it
//     ambiguous between 3.02 and 3.20.
//   - The major component must be at least 2. NMEA 1.x predates every
//     sentence in this library, and accepting "1.00" would let a receiver's
//     product version masquerade as a standard revision.
//
// A build number such as "1000" is not in the dotted form at all, so it is
// rejected regardless.
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, false
	}
	major, minor, found := strings.Cut(s, ".")
	if !found {
		return Version{}, false
	}
	if len(minor) != 2 {
		return Version{}, false
	}
	maj, err := parseSmallInt(major)
	if err != nil {
		return Version{}, false
	}
	min, err := parseSmallInt(minor)
	if err != nil {
		return Version{}, false
	}
	if maj < 2 || maj > 9 {
		return Version{}, false
	}
	return Version{Major: maj, Minor: min}, true
}

func parseSmallInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("%q is not numeric", s)
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}
