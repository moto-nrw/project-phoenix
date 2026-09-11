package config

import (
	"encoding/json"
	"fmt"
	"time"
)

const calendarDateLayout = "2006-01-02"

// CalendarDate is an ISO calendar day used by the legacy workforce models in
// this package. Its string representation is safe to bind to PostgreSQL DATE:
// unlike time.Time, it carries no instant that BUN can convert to UTC.
type CalendarDate string

func NewCalendarDate(year int, month time.Month, day int) CalendarDate {
	return CalendarDate(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(calendarDateLayout))
}

func CalendarDateFromTime(value time.Time) CalendarDate {
	return NewCalendarDate(value.Year(), value.Month(), value.Day())
}

func ParseCalendarDate(value string) (CalendarDate, error) {
	parsed, err := time.Parse(calendarDateLayout, value)
	if err != nil {
		return "", fmt.Errorf("invalid calendar date %q: %w", value, err)
	}
	return CalendarDateFromTime(parsed), nil
}

func (d CalendarDate) String() string {
	if d.IsZero() {
		return "0000-00-00"
	}
	return string(d)
}

func (d CalendarDate) IsZero() bool                   { return d == "" }
func (d CalendarDate) Before(other CalendarDate) bool { return d < other }
func (d CalendarDate) After(other CalendarDate) bool  { return d > other }

func (d CalendarDate) AddDays(days int) CalendarDate {
	return CalendarDateFromTime(d.UTCMidnight().AddDate(0, 0, days))
}

func (d CalendarDate) DaysUntil(other CalendarDate) int {
	return int(other.UTCMidnight().Sub(d.UTCMidnight()) / (24 * time.Hour))
}

func (d CalendarDate) Weekday() time.Weekday { return d.UTCMidnight().Weekday() }

func (d CalendarDate) UTCMidnight() time.Time {
	value, _ := time.Parse(calendarDateLayout, string(d))
	return value
}

func (d CalendarDate) Format(layout string) string { return d.UTCMidnight().Format(layout) }

func (d CalendarDate) MarshalText() ([]byte, error) { return []byte(d), nil }
func (d *CalendarDate) UnmarshalText(value []byte) error {
	parsed, err := ParseCalendarDate(string(value))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d CalendarDate) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(string(d))
}

func (d *CalendarDate) UnmarshalJSON(value []byte) error {
	if string(value) == "null" {
		*d = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return fmt.Errorf("config.CalendarDate: %w", err)
	}
	return d.UnmarshalText([]byte(text))
}
