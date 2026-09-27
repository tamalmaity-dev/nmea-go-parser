package sentences

import (
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// TXT is the Text Transmission sentence, the standard way a receiver reports
// status to a human. It carries a message split across as many sentences as
// needed, which is why the sequence numbers are part of the format rather than
// an afterthought.
//
//	$GNTXT,01,01,02,u-blox AG - www.u-blox.com
//	$GNTXT,01,02,02,-1.10 hPa
//
// Field layout:
//
//	0 total messages  01-99, how many this text is split across
//	1 message number  01-99, which one this is
//	2 message id      01-99
//	3+ text           the payload, ASCII
//
// The message id is defined by the standard only as a number. u-blox and some
// other vendors use a sub-coding where 01 is an error, 02 a warning, 03 a
// notice, and 07 a user message; that is a vendor convention, not the
// standard, so MessageID keeps the raw number and a helper offers the u-blox
// reading without asserting it.
type TXT struct {
	Base
	// TotalMessages is how many sentences this text is split across, and
	// MessageNumber is which one this is.
	TotalMessages int
	MessageNumber int
	MessageID     int
	// Text is the payload of this fragment, with padding trimmed.
	Text string
}

// Assembled returns the whole message once every fragment has arrived, and
// false while any are still missing. A caller feeds each sentence to an
// accumulator and checks completeness.
func (t TXT) Assembled() (string, bool) {
	return t.Text, t.MessageNumber == t.TotalMessages
}

// TextAccumulator reassembles a TXT message from its fragments.
//
// A TXT message is only meaningful as a whole, and a program that logs each
// fragment separately produces unreadable output, so this buffers until the
// sequence is complete.
type TextAccumulator struct {
	total   int
	parts   map[int]string
	order   []int
	started bool
}

// Add folds one fragment in and returns the complete message once the last
// fragment has arrived, repeating the last result if called again with a
// duplicate.
func (a *TextAccumulator) Add(t TXT) (message string, complete bool) {
	if t.MessageNumber < 1 {
		return "", false
	}
	if !a.started {
		a.started = true
		a.total = t.TotalMessages
		a.parts = make(map[int]string, max(t.TotalMessages, 1))
	}
	// A new sequence resets the accumulator, so two messages in a row do not
	// get spliced together.
	if t.TotalMessages != a.total {
		a.total = t.TotalMessages
		a.parts = make(map[int]string, t.TotalMessages)
		a.order = a.order[:0]
	}
	if _, seen := a.parts[t.MessageNumber]; !seen {
		a.order = append(a.order, t.MessageNumber)
	}
	a.parts[t.MessageNumber] = t.Text

	if t.TotalMessages < 1 || len(a.parts) < t.TotalMessages {
		return "", false
	}

	var b strings.Builder
	for _, n := range a.order {
		b.WriteString(a.parts[n])
	}
	return b.String(), true
}

// Reset empties the accumulator.
func (a *TextAccumulator) Reset() {
	a.total, a.parts, a.order, a.started = 0, nil, nil, false
}

// TextMessageID names the u-blox sub-coding of the TXT message id, which is
// a vendor convention rather than part of the standard. An unrecognised value
// is reported as unknown rather than guessed.
func TextMessageID(id int) string {
	switch id {
	case 1:
		return "error"
	case 2:
		return "warning"
	case 3:
		return "notice"
	case 7:
		return "user"
	default:
		return "unknown"
	}
}

type txt struct{}

func (txt) Formatter() string { return "TXT" }

func (txt) Decode(s nmea.Sentence) (any, error) {
	// The three header fields are required; the text may be blank.
	if !s.HasFields(3) {
		return nil, needFields(s, 3)
	}
	out := TXT{Base: newBase(s)}

	var err error
	if out.TotalMessages, err = s.Int(0); err != nil {
		return out, err
	}
	if out.MessageNumber, err = s.Int(1); err != nil {
		return out, err
	}
	if out.MessageID, err = s.Int(2); err != nil {
		return out, err
	}
	if out.TotalMessages < 1 || out.MessageNumber < 1 ||
		out.MessageNumber > out.TotalMessages {
		return out, errRange("TXT", "message numbering", 0, "1..99 with number <= total")
	}

	// The payload is everything from field 3 onwards, rejoined with commas
	// so text containing a comma survives the round trip.
	out.Text = text(strings.Join(s.Tail(3), ","))
	return out, nil
}
