// Package timetable — shared date-parsing helpers. Used by WP-B11 (student
// day/week), WP-B12 (gaps, substitute), and WP-B13 (exception conflicts)
// handlers.
package timetable

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// berlinDate parses a YYYY-MM-DD input into a calendar date. timezone.Date
// carries no instant, so the historical 00:00–02:00 CET/UTC anchoring pitfall
// cannot occur by construction.
func berlinDate(input string) (timezone.Date, error) {
	return timezone.ParseDate(input)
}

// inclusiveDayCount returns the number of calendar days in the inclusive
// [from, to] range. Date arithmetic is UTC-anchored, so DST transitions
// (23h/25h Berlin days) can never skew the count.
func inclusiveDayCount(from, to timezone.Date) int {
	return from.DaysUntil(to) + 1
}
