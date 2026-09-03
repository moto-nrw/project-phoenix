package calendar

import (
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

var berlin = mustBerlinLocation()

// Date is a calendar-owned DATE value. Its YYYY-MM-DD representation carries
// no instant, so Bun cannot shift it across a timezone boundary.
type Date string

func NewDate(year int, month time.Month, day int) Date {
	return Date(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(dateLayout))
}

func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil || parsed.Format(dateLayout) != value {
		return "", fmt.Errorf("invalid calendar date %q", value)
	}
	return Date(value), nil
}

func (d Date) String() string         { return string(d) }
func (d Date) IsZero() bool           { return d == "" }
func (d Date) Before(other Date) bool { return d < other }
func (d Date) After(other Date) bool  { return d > other }
func (d Date) AddDays(days int) Date {
	value := d.UTCMidnight().AddDate(0, 0, days)
	return NewDate(value.Year(), value.Month(), value.Day())
}
func (d Date) DaysUntil(other Date) int {
	return int(other.UTCMidnight().Sub(d.UTCMidnight()) / (24 * time.Hour))
}
func (d Date) Weekday() time.Weekday        { return d.UTCMidnight().Weekday() }
func (d Date) Year() int                    { return d.UTCMidnight().Year() }
func (d Date) Month() time.Month            { return d.UTCMidnight().Month() }
func (d Date) Day() int                     { return d.UTCMidnight().Day() }
func (d Date) Format(layout string) string  { return d.UTCMidnight().Format(layout) }
func (d Date) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

func (d Date) UTCMidnight() time.Time {
	parsed, _ := time.Parse(dateLayout, d.String())
	return parsed
}

func (d Date) BerlinMidnight() time.Time {
	parsed := d.UTCMidnight()
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, berlin)
}

func (d Date) EndOfDay() time.Time {
	parsed := d.UTCMidnight()
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, berlin)
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

func (d *Date) UnmarshalText(value []byte) error {
	parsed, err := ParseDate(string(value))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d *Date) UnmarshalJSON(value []byte) error {
	if string(value) == "null" {
		*d = ""
		return nil
	}
	var raw string
	if err := json.Unmarshal(value, &raw); err != nil {
		return err
	}
	return d.UnmarshalText([]byte(raw))
}

func mustBerlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}

func normalizeWallClock(value time.Time) time.Time {
	return time.Date(1, time.January, 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}
