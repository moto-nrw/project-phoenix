// Package timetable — shared date-parsing helpers. Used by WP-B11 (student
// day/week) and WP-B12 (gaps, substitute) handlers.
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
