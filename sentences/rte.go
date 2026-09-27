package sentences

import (
	"fmt"
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
	"github.com/tamalmaity-dev/nmea-go-parser/fault"
)

// RTE is the Routes sentence. It carries a list of waypoint names, either as
// a whole route or split across several sentences.
//
//	$GPRTE,2,c,c,0,PBRCPK,PBRTO,PTBR,PPBR*35
//	$GPRTE,1,1,c,0*07
//
// Field layout, per the standard:
//
//	0 total messages   how many RTE sentences make up this route
//	1 message number   which one this is, 1-based
//	2 mode             c complete route, w working route
//	3 route id         identifies the route
//	4+ waypoint names  variable length
//
// Field 3 is the route identifier, not a waypoint. Reading it as one puts a
// bogus entry at the head of every route, which is why RouteID is kept
// separate from Waypoints here.
//
// The load, delete, and list variants of the format reuse the mode field for
// a form code rather than a completeness code. Those are reported with
// IsRequest set and the route name in Target, so a caller can tell a route
// report from a command before acting on it: treating a delete request as a
// route report, or the reverse, corrupts either the receiver's route table
// or the caller's idea of the active route.
type RTE struct {
	Base
	// TotalMessages and MessageNumber split a long route across sentences.
	TotalMessages int
	MessageNumber int
	// Mode is the route completeness: complete or working. It is
	// ModeUnknown when the field held a command form code instead.
	Mode RouteMode
	// RouteID identifies the route this sentence belongs to.
	RouteID string
	// Waypoints are the names in route order, starting at field 4.
	Waypoints []string
	// Target names the route a command sentence acts on. It is empty for a
	// plain route report.
	Target string
	// IsRequestFlag is true when this sentence is a command rather than a
	// report of the route.
	IsRequestFlag bool
}

// IsRequest reports whether the sentence is a command rather than a route
// report. Check this before treating the waypoint list as the active route.
func (r *RTE) IsRequest() bool { return r.IsRequestFlag }

// RouteMode is the RTE mode field: whether the listed waypoints are the whole
// route or just the part currently being followed.
type RouteMode int

const (
	// ModeUnknown is a blank field or a command form code rather than a
	// standard completeness code.
	ModeUnknown RouteMode = iota
	// ModeComplete means all waypoints of the route are listed.
	ModeComplete
	// ModeWorking means the waypoint just left, the one being headed to,
	// and then the remainder.
	ModeWorking
)

func (m RouteMode) String() string {
	switch m {
	case ModeComplete:
		return "complete"
	case ModeWorking:
		return "working"
	default:
		return "unknown"
	}
}

// ParseRouteMode interprets the standard mode field. Anything other than c or
// w is a form code from one of the command variants, reported as
// ModeUnknown with ok false so the caller can inspect the raw text.
func ParseRouteMode(s string) (m RouteMode, ok bool) {
	switch text(s) {
	case "c", "C":
		return ModeComplete, true
	case "w", "W":
		return ModeWorking, true
	default:
		return ModeUnknown, false
	}
}

type rte struct{}

func (rte) Formatter() string { return "RTE" }

func (rte) Decode(s nmea.Sentence) (any, error) {
	// Fields 0 to 3 are the message header and the route ID. Without them
	// the waypoint list cannot be attributed to a route at all.
	if !s.HasFields(4) {
		return nil, needFields(s, 4)
	}
	out := RTE{Base: newBase(s)}

	var err error
	if out.TotalMessages, err = s.Int(0); err != nil {
		return out, err
	}
	if out.MessageNumber, err = s.Int(1); err != nil {
		return out, err
	}
	if out.MessageNumber < 1 {
		return out, fmt.Errorf("sentences: RTE message number %d must be 1 or more: %w",
			out.MessageNumber, nmea.ErrFieldRange)
	}
	if out.TotalMessages > 0 && out.MessageNumber > out.TotalMessages {
		return out, fmt.Errorf("sentences: RTE message %d of %d is out of sequence: %w",
			out.MessageNumber, out.TotalMessages, nmea.ErrFieldValue)
	}

	mode, standard := ParseRouteMode(s.Field(2))
	out.Mode = mode
	out.IsRequestFlag = !standard

	rest := s.Tail(4)
	// In the command variants the route name the command acts on sits where
	// a route ID would be, so it is captured as the target rather than
	// mistaken for a waypoint.
	if out.IsRequestFlag {
		if len(rest) > 0 {
			out.Target = text(rest[0])
			rest = rest[1:]
		}
	} else {
		out.RouteID = text(s.Field(3))
	}

	out.Waypoints = make([]string, 0, len(rest))
	for _, f := range rest {
		if name := text(f); name != "" {
			out.Waypoints = append(out.Waypoints, name)
		}
	}
	return out, nil
}

// CycleComplete reports whether this sentence is the last of its RTE cycle,
// which is the point at which the waypoint list is the whole route.
func (r *RTE) CycleComplete() bool {
	return r.TotalMessages > 0 && r.MessageNumber >= r.TotalMessages
}

// Encode renders a route as one or more RTE sentences with correct checksums,
// splitting the waypoint list so that no sentence exceeds the NMEA 0183
// length limit. Pass an empty talker for the generic "GP".
//
// The split is measured rather than predicted. The header length depends on
// how many sentences there end up being, and that number is not known until
// the split is chosen, so a single-pass budget calculation is off by a byte
// or two. Instead the candidate split is built and checked, and the budget is
// tightened until every sentence genuinely fits.
func (r *RTE) Encode(talker string) []string {
	if talker == "" {
		talker = "GP"
	}
	if len(r.Waypoints) == 0 {
		return nil
	}

	routeID := r.RouteID
	if routeID == "" {
		routeID = "0"
	}

	// A generous starting budget: the fixed header, the checksum, and a
	// little slack for a longer route id or a two-digit message count.
	budget := fault.MaxSentenceLength - len(talker) - len("RTE,00,00,c,") - len(routeID) - 2 - 3

	for attempt := 0; attempt < 8; attempt++ {
		sentences := r.encodeChunks(talker, routeID, budget)
		if longest(sentences) <= fault.MaxSentenceLength {
			return sentences
		}
		// Shrink by the worst overshoot plus a byte of margin, so the loop
		// converges in a couple of passes instead of one byte at a time.
		budget -= fault.MaxSentenceLength - longest(sentences) + 1
		if budget < 8 {
			// A single waypoint name longer than the limit cannot be made to
			// fit. Return the best attempt rather than looping forever or
			// emitting nothing; the caller can see the problem in the output.
			return r.encodeChunks(talker, routeID, 8)
		}
	}
	return r.encodeChunks(talker, routeID, 8)
}

// encodeChunks groups the waypoint list so each group fits the budget, then
// renders one sentence per group.
func (r *RTE) encodeChunks(talker, routeID string, budget int) []string {
	var chunks [][]string
	current := []string{}
	used := 0
	for _, w := range r.Waypoints {
		cost := len(w) + 1
		if len(current) > 0 && used+cost > budget {
			chunks = append(chunks, current)
			current, used = []string{}, 0
		}
		current = append(current, w)
		used += cost
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	out := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		// Field order is total messages first, then this sentence's number.
		// Transposing the two produces a sentence a receiver rejects, because
		// it reads the number as a count and finds it out of range.
		body := talker + "RTE," + itoa(len(chunks)) + "," + itoa(i+1) + ",c," +
			routeID + "," + strings.Join(chunk, ",")
		out = append(out, nmea.Frame(body))
	}
	return out
}

func longest(sentences []string) int {
	max := 0
	for _, s := range sentences {
		if len(s) > max {
			max = len(s)
		}
	}
	return max
}

func itoa(v int) string { return fmt.Sprintf("%d", v) }
