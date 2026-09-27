package sentences

import (
	"fmt"
	"strings"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// RPM is the Revolutions sentence: engine or shaft speed, and propeller pitch.
//
//	$IIRPM,S,1,1200.0,10.5,A
//	$IIRPM,E,2,850.0,,V
//
// Field layout:
//
//	0 source        S = shaft, E = engine
//	1 number        the engine or shaft number
//	2 speed         revolutions per minute
//	3 pitch         percent of maximum, a leading "-" meaning astern
//	4 status        A = valid, V = invalid
//
// The source field is the one that gets skipped. A shaft and the engine driving
// it turn at different speeds, so "1200 RPM" is not a meaningful figure until
// you know which one was meant, and a decoder that ignored the field would
// report the wrong number for every boat whose shaft RPM is not the engine RPM.
type RPM struct {
	Base
	// Source is S for a shaft and E for an engine.
	Source string
	// Number is the engine or shaft number, which pairs with Source.
	Number    int
	HasNumber bool
	// Speed is revolutions per minute.
	Speed    float64
	HasSpeed bool
	// Pitch is percent of maximum. It is signed: a leading "-" means the
	// propeller is going astern, which is a different operating state and not
	// a small negative trim.
	Pitch    float64
	HasPitch bool
	// Astern reports the negative pitch flag, which is the sign of Pitch.
	Astern bool
	// Status is A for valid and V for invalid. An invalid reading still arrives
	// with numbers in it, which is exactly why the flag exists.
	Status nmea.StatusFlag
}

// IsShaft and IsEngine report which the figures describe.
func (r RPM) IsShaft() bool  { return r.Source == "S" }
func (r RPM) IsEngine() bool { return r.Source == "E" }

// Label names the unit the sentence describes, such as "port engine", for a
// display that has to distinguish several.
//
// A receiver that omits the number gets the bare name rather than a "0", which
// would read as engine zero rather than as an absence.
func (r RPM) Label() string {
	side := ""
	if r.HasNumber {
		side = " " + itoa(r.Number)
	}
	switch {
	case r.IsShaft():
		return "shaft" + side
	case r.IsEngine():
		return "engine" + side
	default:
		return "rpm" + side
	}
}

// String renders the reading for a log line.
func (r RPM) String() string {
	var b strings.Builder
	b.WriteString(r.Label())
	if r.HasSpeed {
		fmt.Fprintf(&b, " %.0frpm", r.Speed)
	}
	if r.HasPitch {
		if r.Astern {
			fmt.Fprintf(&b, " %.0f%% astern", -r.Pitch)
		} else {
			fmt.Fprintf(&b, " %.0f%%", r.Pitch)
		}
	}
	if r.Status == 'V' {
		b.WriteString(" (invalid)")
	}
	return b.String()
}

type rpm struct{}

func (rpm) Formatter() string { return "RPM" }

func (rpm) Decode(s nmea.Sentence) (any, error) {
	out := RPM{Base: newBase(s)}
	// The source is the first thing to identify, so an empty sentence is a
	// failure rather than a reading with nothing in it.
	if !s.HasFields(1) {
		return out, needFields(s, 1)
	}

	out.Source = text(s.Field(0))
	// The source is validated because a mislabelled one is not cosmetic: it
	// decides which reading a number belongs to. The unit letters that follow
	// the numbers are not validated, because they are fixed by the format.
	switch out.Source {
	case "S", "E":
	default:
		return out, errMislabeled("RPM", "source", out.Source)
	}

	var err error
	if out.Number, out.HasNumber, err = optionalInt(s, 1); err != nil {
		return out, err
	}
	if out.Speed, out.HasSpeed, err = optionalFloat(s, 2); err != nil {
		return out, err
	}
	if out.Pitch, out.HasPitch, err = optionalFloat(s, 3); err != nil {
		return out, err
	}
	// A negative pitch is the astern flag, not a negative pitch setting. The
	// magnitude is what a display wants, so it is reported unsigned and the
	// direction separately.
	if out.HasPitch && out.Pitch < 0 {
		out.Astern = true
		out.Pitch = -out.Pitch
	}
	out.Status = nmea.ParseStatusFlag(s.Field(4))
	return out, nil
}

// ftoa formats a heading without a trailing run of zeros, for the HSC and RPM
// log lines. The integer helper itoa already exists in this package.
func ftoa(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".")
}
