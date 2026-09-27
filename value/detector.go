package value

import (
	"strconv"
	"sync"
	"time"
)

// VersionReporter is implemented by a decoded sentence that states which
// revision of the standard the receiver speaks outright, which today means
// only VER. A reported version always beats an inferred one.
type VersionReporter interface {
	NMEAVersion() (Version, bool)
}

// FeatureEvidence is implemented by a decoded sentence whose mere presence
// proves the receiver implements a particular feature: a GSV carrying a
// signal id proves NMEA 4.10 or later, and so on.
//
// This is how version detection stays out of the parser. The detector needs
// to know that some sentence proves a feature, but it must not need to know
// what any sentence type is called, or the root package would have to import
// the sentences package and the dependency would run in a circle. A decoder
// therefore simply declares what its presence implies.
type FeatureEvidence interface {
	NMEASupports() []Feature
}

// VersionDetector watches a live stream and works out which revision of the
// standard the receiver speaks, without being told.
//
// Almost no receiver reports its standard version, but the shape of the
// sentences gives it away: a GSV with a trailing signal id is 4.10 or later (4.10+),
// a GSA with a system id likewise, and an RMC with a navigational status is
// 4.10. Inferring the version is therefore reliable enough to gate decoding
// decisions, and far more robust than requiring configuration.
//
// A detector is safe for concurrent use.
type VersionDetector struct {
	mu sync.Mutex

	// minimum is the version assumed before any evidence arrives. NMEA 2.30
	// is the floor because it is the first revision with the FAA mode field,
	// and assuming older would misread the field layout.
	minimum Version

	reported   Version
	hasReport  bool
	inferred   Version
	evidence   map[Feature]string // feature -> the sentence that proved it
	lastUpdate time.Time
	sentenceCt int
}

// NewVersionDetector returns a detector. Pass a minimum to override the
// floor; pass the zero Version for the default of NMEA 2.30.
func NewVersionDetector(minimum Version) *VersionDetector {
	if (minimum == Version{}) {
		minimum = V2_30
	}
	return &VersionDetector{
		minimum:  minimum,
		inferred: minimum,
		evidence: make(map[Feature]string),
	}
}

// Observe feeds one decoded sentence value in, along with the address it
// arrived under. Only the interfaces above are consulted, so this works for
// any sentence type, present or future.
//
// The value is taken as an any rather than as the parser's Event type, so
// this package stays independent of the parser: the detector learns what a
// sentence proves, never how sentences are carried.
func (d *VersionDetector) Observe(value any, address string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.sentenceCt++
	d.lastUpdate = time.Now()

	// A sentence that states the version outright beats any inference.
	if vr, ok := value.(VersionReporter); ok {
		if v, reported := vr.NMEAVersion(); reported {
			d.reported, d.hasReport, d.inferred = v, true, v
		}
	}

	fe, ok := value.(FeatureEvidence)
	if !ok {
		return
	}
	for _, f := range fe.NMEASupports() {
		d.note(f, address)
	}
}

// note records that a feature was seen and raises the inferred version to
// whichever revision introduced it.
func (d *VersionDetector) note(f Feature, by string) {
	if _, seen := d.evidence[f]; seen {
		return
	}
	d.evidence[f] = by
	if v := FeatureIntroduced(f); v.AtLeast(d.inferred) {
		d.inferred = v
	}
}

// Version returns the best available answer and whether it was stated or
// merely inferred.
func (d *VersionDetector) Version() (v Version, reported bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.hasReport {
		return d.reported, true
	}
	return d.inferred, false
}

// Inferred returns the version deduced from the traffic, ignoring any
// reported version. It never returns less than the configured minimum.
func (d *VersionDetector) Inferred() Version {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.inferred
}

// Minimum returns the configured floor.
func (d *VersionDetector) Minimum() Version {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.minimum
}

// HasFeature reports whether a feature has been seen on the wire, and
// returns the address of the sentence that proved it.
func (d *VersionDetector) HasFeature(f Feature) (seen bool, by string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	by, seen = d.evidence[f]
	return seen, by
}

// Sentences reports how many sentences the detector has seen.
func (d *VersionDetector) Sentences() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sentenceCt
}

// String describes the detected version for a log line, making it obvious
// whether the answer came from the receiver or from inference.
func (d *VersionDetector) String() string {
	v, reported := d.Version()
	src := "inferred"
	if reported {
		src = "reported by receiver"
	}
	s := "NMEA " + v.String() + " (" + src + ", from " + strconv.Itoa(d.Sentences()) + " sentences)"
	for f := range d.evidence {
		s += "; " + FeatureName(f) + " seen"
	}
	return s
}
