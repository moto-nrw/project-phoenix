package timezone

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Date aliases the canonical shared-kernel calendar date for existing callers.
type Date = calendar.Date

// NewDate constructs a date, normalizing out-of-range components.
func NewDate(year int, month time.Month, day int) Date { return calendar.NewDate(year, month, day) }

// DateFromTime returns the Berlin calendar day containing t.
func DateFromTime(t time.Time) Date { return calendar.DateFromTime(t) }

// TodayDate returns today's Berlin calendar day.
func TodayDate() Date { return calendar.TodayDate() }

// CalendarDateClock converts an optional instant clock into a calendar clock.
func CalendarDateClock(clocks ...func() time.Time) func() Date {
	return calendar.CalendarDateClock(clocks...)
}

// ParseDate parses a strict YYYY-MM-DD date.
func ParseDate(s string) (Date, error) { return calendar.ParseDate(s) }
