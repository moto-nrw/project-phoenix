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
// Use this instead of time.Now().Truncate(24 * time.Hour) to avoid timezone bugs.
func Today() time.Time {
	return DateOf(time.Now())
}

// DateOf extracts the date portion of a timestamp in Berlin timezone.
// Returns midnight of that date in Berlin timezone.
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

// DateOfUTC extracts the date portion of a timestamp in Berlin timezone
// but returns it as UTC midnight. This is useful for database DATE column
// comparisons where the driver converts timestamps to UTC before comparing.
//
// Example:
//
//	t := time.Date(2026, 1, 18, 0, 30, 0, 0, time.UTC) // 00:30 UTC = 01:30 CET
//	date := timezone.DateOfUTC(t) // 2026-01-18 00:00:00 UTC
func DateOfUTC(t time.Time) time.Time {
	inBerlin := t.In(Berlin)
	return time.Date(
		inBerlin.Year(),
		inBerlin.Month(),
		inBerlin.Day(),
		0, 0, 0, 0,
		time.UTC,
	)
}
