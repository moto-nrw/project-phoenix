package active

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// The retained rows carry calendar dates as the canonical timezone.Date. The
// retained repositories parse and compare those dates through these names so
// they depend on the row vocabulary they persist, not on the calendar package
// itself (#3214). Every entry goes with the last retained row that uses it.

// Date is the calendar date the retained rows store in DATE columns.
type Date = timezone.Date

var (
	// ParseDate reads a YYYY-MM-DD wire value into a Date.
	ParseDate = timezone.ParseDate
	// CalendarDateClock derives today's Berlin calendar date from an optional clock.
	CalendarDateClock = timezone.CalendarDateClock
)
