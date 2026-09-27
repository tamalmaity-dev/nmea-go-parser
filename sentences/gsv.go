package sentences

import (
	"fmt"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// SatellitesInViewPerSentence is how many satellites one GSV sentence can
// describe. Each satellite takes four fields, and the format reserves 17
// fields after the header, so four is the maximum.
const SatellitesInViewPerSentence = 4

// GSV is the Satellites in View sentence. It lists every satellite the
// receiver can hear, along with its elevation, azimuth, and signal strength,
// regardless of whether it is used in the solution.
//

//------------------------------------------------------------------------------------------
//	$GPGSV,2,1,05,01,40,083,46,02,17,308,41,12,07,344,39,14,22,228,45*75
//	$GPGSV,2,2,05,25,12,090,32
//------------------------------------------------------------------------------------------

// Field layout:
//
//	0 total messages   how many GSV sentences make up this cycle
//	1 message number   which one this is, 1-based
//	2 satellites seen  total in view across all messages
//	3+ per satellite:  PRN, elevation degrees, azimuth degrees true, SNR dB
//	n  signal id       GNSS signal, NMEA 4.10 and later, optional
//
// The signal id is ONE field for the whole sentence, not one per satellite,
// and it sits immediately before the checksum. That is the detail that
// matters here: reading it as a fifth value in each satellite group shifts
// every group after the first, which produces plausible elevations and
// azimuths for the wrong satellites.
//
// A GSV cycle is split across several sentences, so a handler that wants the
// whole constellation should accumulate by cycle rather than act on each
// sentence alone. SNR of 0 means the satellite is tracked but has no usable
// signal, which is different from a satellite that is not tracked at all.
type GSV struct {
	Base
	// TotalMessages is how many sentences make up this cycle, and
	// MessageNumber is which of them this is. Equal values mean the cycle
	// is complete.
	TotalMessages int
	MessageNumber int
	// SatellitesInView is the count for the whole cycle, not for this
	// sentence.
	SatellitesInView int
	// Satellites are the satellites described by this sentence.
	Satellites []nmea.Satellite
	// SignalID is the GNSS signal for this sentence, NMEA 4.10 and later.
	// One value covers every satellite in the sentence.
	SignalID    nmea.SignalDigit
	HasSignalID bool
}

type gsv struct{}

func (gsv) Formatter() string { return "GSV" }

func (gsv) Decode(s nmea.Sentence) (any, error) {
	// Header is three fields; anything past that is satellite data in
	// groups of four.
	if !s.HasFields(3) {
		return nil, needFields(s, 3)
	}
	out := GSV{Base: newBase(s)}

	var err error
	if out.TotalMessages, err = s.Int(0); err != nil {
		return out, err
	}
	if out.MessageNumber, err = s.Int(1); err != nil {
		return out, err
	}
	if out.SatellitesInView, err = s.Int(2); err != nil {
		return out, err
	}
	if out.MessageNumber < 1 || (out.TotalMessages > 0 && out.MessageNumber > out.TotalMessages) {
		return out, fmt.Errorf(
			"sentences: GSV message %d of %d is out of sequence: %w",
			out.MessageNumber, out.TotalMessages, nmea.ErrFieldValue)
	}

	// Four fields per satellite. Receivers pad the last sentence of a cycle
	// with empty fields, and the published GSV examples do too, so trailing
	// blanks are dropped before anything else is counted. A signal id is a
	// single hexadecimal digit and is never blank, so this cannot swallow one.
	payload := len(s.Fields) - 3
	for payload > 0 && s.Blank(3+payload-1) {
		payload--
	}
	remainder := payload % SatellitesInViewPerSentence

	hasSignal := remainder == 1 && isSignalID(s.Field(3+payload-1))
	switch {
	case remainder == 0:
	case hasSignal:
		payload--
	default:
		// A partial group is only tolerated when every field in it is blank.
		// Some receivers pad the last group of a cycle with empty fields, and
		// the published GSV examples do too, but a partial group carrying
		// data means the sentence was genuinely truncated and the fields
		// that did arrive cannot be trusted.
		for i := 3 + payload - remainder; i < len(s.Fields); i++ {
			if !s.Blank(i) {
				return out, fmt.Errorf(
					"sentences: GSV ends with a partial satellite group holding %q: %w",
					s.Field(i), nmea.ErrFieldCount)
			}
		}
		payload -= remainder
	}

	groups := payload / SatellitesInViewPerSentence

	// The signal id is one field for the whole sentence, not one per
	// satellite, so it is read before the satellites are built and stamped
	// onto each of them.
	var signal nmea.SignalDigit
	hasSignalID := false
	if hasSignal {
		if id, ok := nmea.ParseSignalID(s.Field(3 + groups*SatellitesInViewPerSentence)); ok {
			signal, hasSignalID = id, true
		}
	}

	// In multi-GNSS operation the GSV talker names the system being reported,
	// so it attributes every satellite in the sentence. A GN talker means a
	// fused set the receiver is not attributing, and a talker this library
	// does not know means the constellation genuinely cannot be named.
	constellation := nmea.TalkerConstellation(s.Talker)
	constellationKnown := constellation != nmea.ConstellationUnknown

	out.Satellites = make([]nmea.Satellite, 0, groups)
	for g := 0; g < groups; g++ {
		base := 3 + g*SatellitesInViewPerSentence
		sat, err := decodeSatellite(s, base)
		if err != nil {
			return out, err
		}
		sat.Constellation = constellation
		sat.ConstellationKnown = constellationKnown
		sat.Signal, sat.HasSignal = signal, hasSignalID
		out.Satellites = append(out.Satellites, sat)
	}

	out.SignalID, out.HasSignalID = signal, hasSignalID
	return out, nil
}

// isSignalID reports whether a field looks like a NMEA 4.10 signal id, which
// is a single hexadecimal digit.
//
// Testing the shape rather than just the position is what makes this safe: a
// four-field GSV with one field missing would otherwise be read as a
// four-satellite sentence with a signal id, quietly losing a satellite.
func isSignalID(field string) bool {
	if len(field) != 1 {
		return false
	}
	c := field[0]
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

// NMEASupports implements nmea.FeatureEvidence: a GSV carrying a signal id
// proves the receiver speaks NMEA 4.10 or later.
func (g GSV) NMEASupports() []nmea.Feature {
	if g.HasSignalID {
		return nmea.FeatureOnly(nmea.FeatureSignalID)
	}
	return nil
}

// decodeSatellite reads one four-field satellite group starting at index
// base.
func decodeSatellite(s nmea.Sentence, base int) (nmea.Satellite, error) {
	var sat nmea.Satellite

	id, err := s.Int(base)
	if err != nil {
		return sat, err
	}
	sat.ID = id

	if sat.Elevation, err = s.Float(base + 1); err != nil {
		return sat, err
	}
	if sat.Azimuth, err = s.Float(base + 2); err != nil {
		return sat, err
	}
	// SNR is the only optional field in the group: a receiver reports 0 for
	// a tracked satellite with no signal, but leaves the field out when it
	// has no measurement at all.
	if snr, present, err := optionalFloat(s, base+3); err != nil {
		return sat, err
	} else if present {
		sat.SNR, sat.HasSNR = snr, true
	}
	return sat, nil
}

// ApplyFix folds the GSV into the fix state.
//
// A cycle is accumulated across its sentences rather than each one replacing the
// last, because a GSV cycle describes up to four satellites per sentence and a
// receiver with eleven in view sends three of them. Keeping only the most
// recent would report a sky of four when the receiver can see eleven, and a
// satellite plot that silently omits two thirds of the sky is worse than none.
//
// The accumulation is also per system. A multi-GNSS receiver runs a separate
// cycle for each constellation, so a single list would be emptied every time
// the next system started reporting.
func (g GSV) ApplyFix(f *nmea.Fix) {
	c := nmea.TalkerConstellation(g.Talker())
	// SatellitesInView is a required field, so it is always meaningful, and it
	// is passed even for a sentence that lists none, because it describes the
	// cycle rather than the satellites in this one sentence.
	f.SetSatellites(c, c != nmea.ConstellationUnknown, g.Satellites,
		g.MessageNumber <= 1, g.SatellitesInView, true)
}

// CycleComplete reports whether this sentence is the last in its GSV cycle,
// which is the point at which SatellitesInView is a complete picture.
func (g *GSV) CycleComplete() bool {
	return g.TotalMessages > 0 && g.MessageNumber >= g.TotalMessages
}

// Strongest returns the satellite with the highest SNR, which is a quick way
// to see whether the constellation geometry is good enough for a fix.
func (g *GSV) Strongest() (nmea.Satellite, bool) {
	var best nmea.Satellite
	found := false
	for _, sat := range g.Satellites {
		if !sat.HasSNR {
			continue
		}
		if !found || sat.SNR > best.SNR {
			best, found = sat, true
		}
	}
	return best, found
}
