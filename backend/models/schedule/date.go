package schedule

import (
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

var berlin = mustBerlinLocation()

// Date is the schedule model's calendar-day representation. It has no clock
// or timezone, so PostgreSQL DATE values cannot shift during BUN binding.
type Date string

func NewDate(year int, month time.Month, day int) Date {
	return Date(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(dateLayout))
}

func DateFromTime(value time.Time) Date {
	year, month, day := value.In(berlin).Date()
	return NewDate(year, month, day)
}

func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil || parsed.Format(dateLayout) != value {
		return "", fmt.Errorf("invalid calendar date %q", value)
	}
	return Date(value), nil
}

func (d Date) String() string { return string(d) }
func (d Date) IsZero() bool   { return d == "" }

func (d Date) Compare(other interface{ String() string }) int {
	value := other.String()
	switch {
	case string(d) < value:
		return -1
	case string(d) > value:
		return 1
	default:
		return 0
	}
}

func (d Date) Before(other interface{ String() string }) bool { return d.Compare(other) < 0 }
func (d Date) After(other interface{ String() string }) bool  { return d.Compare(other) > 0 }

func (d Date) AddDays(days int) Date {
	return Date(d.UTCMidnight().AddDate(0, 0, days).Format(dateLayout))
}

func (d Date) DaysUntil(other Date) int {
	return int(other.UTCMidnight().Sub(d.UTCMidnight()) / (24 * time.Hour))
}

func (d Date) Weekday() time.Weekday { return d.UTCMidnight().Weekday() }

func (d Date) StartOfISOWeek() Date {
	return d.AddDays(-((int(d.Weekday()) + 6) % 7))
}

func (d Date) BerlinMidnight() time.Time {
	year, month, day := d.components()
	return time.Date(year, month, day, 0, 0, 0, 0, berlin)
}

func (d Date) UTCMidnight() time.Time {
	year, month, day := d.components()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (d Date) EndOfDay() time.Time {
	year, month, day := d.components()
	return time.Date(year, month, day, 23, 59, 59, 0, berlin)
}

func (d Date) Year() int                   { return d.UTCMidnight().Year() }
func (d Date) Month() time.Month           { return d.UTCMidnight().Month() }
func (d Date) Day() int                    { return d.UTCMidnight().Day() }
func (d Date) Format(layout string) string { return d.UTCMidnight().Format(layout) }

func (d Date) MarshalText() ([]byte, error) { return []byte(d), nil }

func (d *Date) UnmarshalText(value []byte) error {
	parsed, err := ParseDate(string(value))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(string(d))
}

func (d *Date) UnmarshalJSON(value []byte) error {
	if string(value) == "null" {
		*d = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return fmt.Errorf("schedule.Date: %w", err)
	}
	return d.UnmarshalText([]byte(text))
}

func (d Date) components() (int, time.Month, int) {
	parsed, err := time.Parse(dateLayout, string(d))
	if err != nil {
		return 0, 0, 0
	}
	return parsed.Year(), parsed.Month(), parsed.Day()
}

func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}

func mustBerlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic("load Europe/Berlin timezone: " + err.Error())
	}
	return location
}
