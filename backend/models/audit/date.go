package audit

import (
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

var berlin = mustBerlinLocation()

// Date is an Audit-owned calendar date. Its string representation prevents
// PostgreSQL DATE values from shifting through UTC conversion.
type Date string

func NewDate(year int, month time.Month, day int) Date {
	return Date(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(dateLayout))
}

func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(dateLayout, value)
	if err != nil || parsed.Format(dateLayout) != value {
		return "", fmt.Errorf("invalid audit calendar date %q", value)
	}
	return Date(value), nil
}

func (d Date) String() string { return string(d) }
func (d Date) IsZero() bool   { return d == "" }

func (d Date) AddDays(days int) Date {
	return Date(d.utcMidnight().AddDate(0, 0, days).Format(dateLayout))
}

func (d Date) BerlinMidnight() time.Time {
	parsed := d.utcMidnight()
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, berlin)
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
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
	parsed, err := ParseDate(raw)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

func (d Date) utcMidnight() time.Time {
	parsed, _ := time.Parse(dateLayout, d.String())
	return parsed
}

func mustBerlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(err)
	}
	return location
}
