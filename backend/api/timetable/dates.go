// Package timetable — shared date-parsing helpers. Used by WP-B11 (student
// day/week), WP-B12 (gaps, substitute), and WP-B13 (exception conflicts)
// handlers.
package timetable

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// berlinDate parses a YYYY-MM-DD input anchored in Berlin timezone. The
// distinction matters for the 00:00–02:00 CET/UTC gap: a UTC-anchored parse
// of "2026-04-22" would be midnight UTC (02:00 Berlin), and a Berlin-DateOf
// round-trip would land on the previous day. Using ParseInLocation sidesteps
// that entirely.
func berlinDate(input string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, input, timezone.Berlin)
}

// inclusiveDayCount returns the number of Berlin-local days in the inclusive
// [from, to] range. Anchors both ends to UTC midnight of the same Berlin date
// via timezone.DateOfUTC so DST transitions (23h/25h days) don't skew the
// count — a plain to.Sub(from).Hours()/24 undercounts in spring and overcounts
// in autumn.
func inclusiveDayCount(from, to time.Time) int {
	fromUTC := timezone.DateOfUTC(from)
	toUTC := timezone.DateOfUTC(to)
	return int(toUTC.Sub(fromUTC).Hours()/24) + 1
}
