package timezone

import (
	"encoding/json"
	"fmt"
	"time"
)

// dateLayout is the wire and storage format for calendar dates.
const dateLayout = "2006-01-02"

// Date is a calendar date encoded as YYYY-MM-DD, with no clock time and
// no timezone. Use it for every value whose business meaning is "a day on
// the calendar" (attendance day, birthday, validity range), in model fields
// mapped to PostgreSQL DATE columns, in repository signatures, and in API
// payloads (it marshals as "YYYY-MM-DD").
//
// Why this type exists: bun converts every time.Time parameter to UTC before
// binding. Date's string representation carries no instant, so PostgreSQL
// DATE values cannot shift across a timezone boundary.
//
// Date is comparable: == works, and it can be used as a map key.
// The zero value means "unset". Optional dates use *Date.
type Date string

// NewDate returns the Date for the given year/month/day. Out-of-range
// components are normalized the same way time.Date normalizes them
// (e.g. NewDate(2026, 1, 32) == NewDate(2026, 2, 1)).
func NewDate(year int, month time.Month, day int) Date {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return Date(t.Format(dateLayout))
}

// DateFromTime returns the calendar date of the instant t in Berlin.
// This is the single conversion from instant to calendar day — it replaces
// both the DateOf-for-TIMESTAMPTZ and DateOfUTC-for-DATE compensation
// helpers for calendar-day logic.
func DateFromTime(t time.Time) Date {
	inBerlin := t.In(Berlin)
	return Date(inBerlin.Format(dateLayout))
}

// TodayDate returns today's calendar date in Berlin.
func TodayDate() Date {
	return DateFromTime(time.Now())
}

// CalendarDateClock converts an optional instant clock to Berlin calendar dates.
func CalendarDateClock(clocks ...func() time.Time) func() Date {
	if len(clocks) == 0 || clocks[0] == nil {
		return TodayDate
	}
	return func() Date { return DateFromTime(clocks[0]()) }
}

// ParseDate parses a strict "YYYY-MM-DD" string.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return "", fmt.Errorf("invalid calendar date %q: %w", s, err)
	}
	if t.Format(dateLayout) != s {
		return "", fmt.Errorf("invalid calendar date %q", s)
	}
	return Date(s), nil
}

// String renders the stored "YYYY-MM-DD" representation without converting
// it through time.Time. Construct dates via NewDate, ParseDate, or DateFromTime.
func (d Date) String() string {
	return string(d)
}

// IsZero reports whether d is the zero value ("unset").
func (d Date) IsZero() bool {
	return d == ""
}

// Compare returns -1 if d is before o, 0 if equal, +1 if after.
func (d Date) Compare(o Date) int {
	switch {
	case d < o:
		return -1
	case d > o:
		return 1
	default:
		return 0
	}
}

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is after o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// AddDays returns the date n calendar days after d (n may be negative).
// The arithmetic runs at UTC midnight, which has no DST, so a Berlin
// 23h/25h day can never produce an off-by-one.
func (d Date) AddDays(n int) Date {
	return Date(d.UTCMidnight().AddDate(0, 0, n).Format(dateLayout))
}

// DaysUntil returns the exact number of calendar days from d to o
// (negative when o is before d). UTC anchoring makes the division exact
// across DST transitions.
func (d Date) DaysUntil(o Date) int {
	return int(o.UTCMidnight().Sub(d.UTCMidnight()) / (24 * time.Hour))
}

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday {
	return d.UTCMidnight().Weekday()
}

// BerlinMidnight returns 00:00:00 Berlin on this date. Use it when a
// TIMESTAMPTZ comparison needs the start-of-day instant.
func (d Date) BerlinMidnight() time.Time {
	year, month, day := d.components()
	return time.Date(year, month, day, 0, 0, 0, 0, Berlin)
}

// UTCMidnight returns 00:00:00 UTC on this date. This matches how DATE
// columns historically scanned back into time.Time, so it is the interop
// accessor for not-yet-migrated call sites.
func (d Date) UTCMidnight() time.Time {
	year, month, day := d.components()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// EndOfDay returns 23:59:59 Berlin on this date. Use it when a TIMESTAMPTZ
// comparison needs the end-of-day instant (mirrors package EndOfDay).
func (d Date) EndOfDay() time.Time {
	year, month, day := d.components()
	return time.Date(year, month, day, 23, 59, 59, 0, Berlin)
}

// Year returns the calendar year.
func (d Date) Year() int { return d.UTCMidnight().Year() }

// Month returns the calendar month.
func (d Date) Month() time.Month { return d.UTCMidnight().Month() }

// Day returns the day of the month.
func (d Date) Day() int { return d.UTCMidnight().Day() }

// Format renders the date with a time.Time layout (date verbs only —
// there is no clock or zone to format). Useful for German display
// formats like d.Format("02.01.2006") in exports.
func (d Date) Format(layout string) string {
	return d.UTCMidnight().Format(layout)
}

// MarshalText implements encoding.TextMarshaler ("YYYY-MM-DD").
func (d Date) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Date) UnmarshalText(b []byte) error {
	parsed, err := ParseDate(string(b))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// MarshalJSON renders the date as "YYYY-MM-DD"; the zero Date as null.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON parses "YYYY-MM-DD" or null.
func (d *Date) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*d = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("timezone.Date: %w", err)
	}
	return d.UnmarshalText([]byte(s))
}

func (d Date) components() (int, time.Month, int) {
	parsed, err := time.Parse(dateLayout, string(d))
	if err != nil {
		return 0, 0, 0
	}
	return parsed.Year(), parsed.Month(), parsed.Day()
}
