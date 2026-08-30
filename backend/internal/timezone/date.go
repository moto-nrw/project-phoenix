package timezone

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// dateLayout is the wire and storage format for calendar dates.
const dateLayout = "2006-01-02"

// Date is a calendar date — a year/month/day triple with no clock time and
// no timezone. Use it for every value whose business meaning is "a day on
// the calendar" (attendance day, birthday, validity range), in model fields
// mapped to PostgreSQL DATE columns, in repository signatures, and in API
// payloads (it marshals as "YYYY-MM-DD").
//
// Why this type exists: bun converts every time.Time parameter to UTC
// before binding (BaseDialect.AppendTime), so a Berlin-midnight time.Time
// written to a DATE column lands one day behind between 00:00 and 02:00
// Berlin time. Date carries no instant, binds as a 'YYYY-MM-DD' literal via
// driver.Valuer, and is therefore immune. See .claude/rules/calendar-dates.md.
//
// Date is comparable: == works, and it can be used as a map key.
// The zero value means "unset" — see Value() for the NULL semantics.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate returns the Date for the given year/month/day. Out-of-range
// components are normalized the same way time.Date normalizes them
// (e.g. NewDate(2026, 1, 32) == NewDate(2026, 2, 1)).
func NewDate(year int, month time.Month, day int) Date {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// DateFromTime returns the calendar date of the instant t in Berlin.
// This is the single conversion from instant to calendar day — it replaces
// both the DateOf-for-TIMESTAMPTZ and DateOfUTC-for-DATE compensation
// helpers for calendar-day logic.
func DateFromTime(t time.Time) Date {
	inBerlin := t.In(Berlin)
	return Date{Year: inBerlin.Year(), Month: inBerlin.Month(), Day: inBerlin.Day()}
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
		return Date{}, fmt.Errorf("invalid calendar date %q: %w", s, err)
	}
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}, nil
}

// String renders the date as "YYYY-MM-DD". It formats the fields directly
// (no time.Time normalization), so an unnormalized literal like
// Date{2026, 13, 40} renders visibly wrong instead of silently shifting —
// construct dates via NewDate/ParseDate/DateFromTime.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, int(d.Month), d.Day)
}

// IsZero reports whether d is the zero value ("unset").
func (d Date) IsZero() bool {
	return d == Date{}
}

// Compare returns -1 if d is before o, 0 if equal, +1 if after.
func (d Date) Compare(o Date) int {
	switch {
	case d.Year != o.Year:
		return sign(d.Year - o.Year)
	case d.Month != o.Month:
		return sign(int(d.Month) - int(o.Month))
	default:
		return sign(d.Day - o.Day)
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
	t := time.Date(d.Year, d.Month, d.Day+n, 0, 0, 0, 0, time.UTC)
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}
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
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, Berlin)
}

// UTCMidnight returns 00:00:00 UTC on this date. This matches how DATE
// columns historically scanned back into time.Time, so it is the interop
// accessor for not-yet-migrated call sites.
func (d Date) UTCMidnight() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// EndOfDay returns 23:59:59 Berlin on this date. Use it when a TIMESTAMPTZ
// comparison needs the end-of-day instant (mirrors package EndOfDay).
func (d Date) EndOfDay() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 23, 59, 59, 0, Berlin)
}

// Format renders the date with a time.Time layout (date verbs only —
// there is no clock or zone to format). Useful for German display
// formats like d.Format("02.01.2006") in exports.
func (d Date) Format(layout string) string {
	return d.UTCMidnight().Format(layout)
}

// Value implements driver.Valuer. The date is bound as the literal string
// "YYYY-MM-DD", which PostgreSQL coerces in DATE position without any
// timezone conversion — the driver has no instant to shift.
//
// The zero Date binds as NULL. On a NOT NULL column that fails loudly
// (instead of silently storing 0001-01-01); in WHERE position a NULL
// comparison matches nothing. An optional date is *Date, never a
// zero-Date sentinel.
func (d Date) Value() (driver.Value, error) {
	if d.IsZero() {
		return nil, nil
	}
	return d.String(), nil
}

// Scan implements sql.Scanner. pgdriver delivers DATE columns as raw
// "YYYY-MM-DD" strings; []byte covers database/sql conversion paths.
// time.Time is accepted defensively (TIMESTAMP-typed expressions, or a
// future driver swap) and is interpreted in Berlin.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = Date{}
		return nil
	case string:
		parsed, err := ParseDate(v)
		if err != nil {
			return fmt.Errorf("timezone.Date: scan %q: %w", v, err)
		}
		*d = parsed
		return nil
	case []byte:
		return d.Scan(string(v))
	case time.Time:
		*d = DateFromTime(v)
		return nil
	default:
		return fmt.Errorf("timezone.Date: cannot scan %T", src)
	}
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
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("timezone.Date: %w", err)
	}
	return d.UnmarshalText([]byte(s))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}
