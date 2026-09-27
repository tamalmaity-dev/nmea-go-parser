package sentences

import (
	"fmt"
	"time"

	"github.com/tamalmaity-dev/nmea-go-parser"
)

// ZDA is the Time and Date sentence, in full numeric form. RMC carries the
// date too, but ZDA does not depend on a fix: a receiver can report the
// date and time while still searching for satellites, which makes ZDA the
// sentence to watch when establishing whether a device has acquired its
// time at all.
//
//	$GPZDA,160012.71,11,03,2004,-1,00*7D
//
// Field layout, per the standard:
//
//	0 UTC time       hhmmss.ss
//	1 day            01-31
//	2 month          01-12
//	3 year           four digits
//	4 local zone     hours from UTC, -13 to +13, may be negative
//	5 local minute   00-59, carrying the same sign as the hours
//
// The year is four digits here, unlike the two-digit year in RMC, and both
// are accepted by nmea.ParseDate. The local zone is signed, and the minutes
// field takes the sign of the hours, so -1 and 00 means one hour behind UTC
// while -1 and 30 means one and a half hours behind.
type ZDA struct {
	Base
	// UTC is the time of day in UTC.
	UTC nmea.TOD
	// Date is the calendar date in UTC.
	Date nmea.Date
	// LocalZoneHours and LocalZoneMinutes are the receiver's offset from
	// UTC. They exist for receivers with a real-time clock; both are usually
	// zero.
	LocalZoneHours   int
	LocalZoneMinutes int
}

type zda struct{}

func (zda) Formatter() string { return "ZDA" }

func (zda) Decode(s nmea.Sentence) (any, error) {
	// A ZDA with nothing but the time is still useful, so only the time is
	// required.
	if !s.HasFields(1) {
		return nil, needFields(s, 1)
	}
	out := ZDA{Base: newBase(s)}

	t, err := s.Time(0)
	if err != nil {
		return out, err
	}
	out.UTC = t

	// Day, month, and year are optional as a group: some receivers emit a
	// time-only ZDA while they have no date. A partial date is not usable,
	// so it is dropped rather than half-applied.
	if s.HasFields(4) && !s.Blank(1) && !s.Blank(2) && !s.Blank(3) {
		day, err := s.Int(1)
		if err != nil {
			return out, err
		}
		month, err := s.Int(2)
		if err != nil {
			return out, err
		}
		year, err := s.Int(3)
		if err != nil {
			return out, err
		}
		// nmea.Date does the calendar validation, including leap years, so
		// the range checks here are only about rejecting nonsense the
		// standard could not have meant.
		if year < 1 {
			return out, fmt.Errorf("sentences: ZDA year %d is not a calendar year: %w",
				year, nmea.ErrFieldRange)
		}
		d, err := nmea.NewDate(year, month, day)
		if err != nil {
			return out, err
		}
		out.Date = d
	}

	if out.LocalZoneHours, _, err = optionalInt(s, 4); err != nil {
		return out, err
	}
	if out.LocalZoneMinutes, _, err = optionalInt(s, 5); err != nil {
		return out, err
	}
	return out, nil
}

// ApplyFix folds the ZDA into the fix state. ZDA contributes only the clock,
// never a position, so it cannot change fix validity.
func (z ZDA) ApplyFix(f *nmea.Fix) {
	if z.UTC.Available {
		f.TimeOfDay = z.UTC
	}
	if z.Date.Available {
		f.Date = z.Date
	}
}

// Timestamp returns the full date and time in UTC, and false when the
// receiver has not sent a complete date.
func (z *ZDA) Timestamp() (t time.Time, ok bool) { return z.Date.Combine(z.UTC) }
