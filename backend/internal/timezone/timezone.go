// Package timezone provides consistent timezone handling for the application.
// All date calculations use Europe/Berlin since the app is only used in Germany.
package timezone

import (
	"time"
)

// Berlin is the timezone for all school operations.
// This is hardcoded since the app is only used in Germany.
var Berlin *time.Location

func init() {
	var err error
	Berlin, err = time.LoadLocation("Europe/Berlin")
	if err != nil {
		// Fallback to UTC+1 if timezone data is not available
		// This should never happen in production but provides safety
		Berlin = time.FixedZone("CET", 1*60*60)
	}
}

// Today returns the current date in Berlin timezone with time set to midnight.
// Use it only for TIMESTAMPTZ boundary instants. For calendar-day values
// (DATE columns, API dates) use TodayDate() — time.Time params are converted
// to UTC by the driver and must never reach a DATE column.
func Today() time.Time {
	return DateOf(time.Now())
}

// DateOf extracts the date portion of a timestamp in Berlin timezone.
// Returns midnight of that date in Berlin timezone — an instant, for
// TIMESTAMPTZ boundary math only. For calendar-day values use DateFromTime().
//
// Example:
//
//	t := time.Date(2026, 1, 18, 0, 30, 0, 0, time.UTC) // 00:30 UTC = 01:30 CET
//	date := timezone.DateOf(t) // 2026-01-18 00:00:00 Europe/Berlin
func DateOf(t time.Time) time.Time {
	inBerlin := t.In(Berlin)
	return time.Date(
		inBerlin.Year(),
		inBerlin.Month(),
		inBerlin.Day(),
		0, 0, 0, 0,
		Berlin,
	)
}

// Now returns the current time in Berlin timezone.
func Now() time.Time {
	return time.Now().In(Berlin)
}

// EndOfDay returns 23:59:59 in Berlin timezone for the calendar date of t.
// Use this when converting a PostgreSQL DATE column (returned as UTC midnight
// by pgx) into a TIMESTAMPTZ end-of-day value.
func EndOfDay(t time.Time) time.Time {
	inBerlin := t.In(Berlin)
	return time.Date(
		inBerlin.Year(), inBerlin.Month(), inBerlin.Day(),
		23, 59, 59, 0, Berlin,
	)
}

// FormatBerlinClock formats a timestamp as "HH:MM" in Europe/Berlin.
// Returns nil when the input is nil so callers can pass straight into
// optional JSON fields without a manual nil check.
func FormatBerlinClock(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.In(Berlin).Format("15:04")
	return &formatted
}

// NormalizeWallClock returns a new time.Time at year 0001-01-01 UTC with t's own
// wall-clock Hour/Minute/Second/Nanosecond preserved in t's existing
// Location.
//
// This is the bridge between SQL TIME columns whose year-anchor the driver
// picks arbitrarily (0000 vs 2000) and downstream consumers that need to
// compare or format the wall-clock portion regardless of the original
// Location or year. Dropping all date/Location data preserves the value the
// admin saw ("08:00") rather than a year-driven .After() mismatch.
//
// Used by the materialization service when normalising timeframe rows for
// insertion into activity_instances.start_time (TIME), by the activities
// schedule repo when reading the same timeframe back for the WP-B13
// conflict endpoint, and by the conflict handler itself when comparing
// the exception's modified start_time against the student's resolved
// arrival.
func NormalizeWallClock(t time.Time) time.Time {
	return WallClockFromTime(t).Time()
}

// SameClockTime compares only the time-of-day components, ignoring the date
// anchor and location a driver may have attached on scan.
func SameClockTime(a, b time.Time) bool {
	return a.Hour() == b.Hour() &&
		a.Minute() == b.Minute() &&
		a.Second() == b.Second() &&
		a.Nanosecond() == b.Nanosecond()
}
