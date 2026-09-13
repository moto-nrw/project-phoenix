package timezone

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Berlin is the canonical school timezone.
var Berlin = calendar.Berlin

// Today returns Berlin midnight for today, for timestamp boundaries only.
func Today() time.Time { return calendar.Today() }

// DateOf returns Berlin midnight for t, for timestamp boundaries only.
func DateOf(t time.Time) time.Time { return calendar.DateOf(t) }

// Now returns the current instant in Berlin.
func Now() time.Time { return calendar.Now() }

// EndOfDay returns 23:59:59 Berlin for t's calendar day.
func EndOfDay(t time.Time) time.Time { return calendar.EndOfDay(t) }

// FormatBerlinClock formats an optional timestamp as HH:MM in Berlin.
func FormatBerlinClock(t *time.Time) *string { return calendar.FormatBerlinClock(t) }

// NormalizeWallClock preserves t's clock components at the canonical UTC anchor.
func NormalizeWallClock(t time.Time) time.Time { return calendar.NormalizeWallClock(t) }

// SameClockTime compares clock components, ignoring date and location.
func SameClockTime(a, b time.Time) bool { return calendar.SameClockTime(a, b) }
