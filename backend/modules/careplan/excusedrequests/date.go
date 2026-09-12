package excusedrequests

import (
	"fmt"
	"time"
)

// ParseDate validates the canonical calendar representation without creating
// an instant that could be shifted by a database driver.
func ParseDate(value string) (Date, error) {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return "", fmt.Errorf("invalid calendar date %q: %w", value, err)
	}
	if parsed.Format(DateLayout) != value {
		return "", fmt.Errorf("invalid calendar date %q", value)
	}
	return Date(value), nil
}

func (d Date) calendarTime() time.Time {
	parsed, err := time.Parse(DateLayout, d.String())
	if err != nil {
		return time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
	}
	return parsed
}

// AddDays uses calendar arithmetic at UTC midnight, independent of DST.
func (d Date) AddDays(days int) Date {
	return Date(d.calendarTime().AddDate(0, 0, days).Format(DateLayout))
}

func (d Date) Weekday() time.Weekday { return d.calendarTime().Weekday() }

func (d Date) StartOfISOWeek() Date {
	return d.AddDays(-((int(d.Weekday()) + 6) % 7))
}

func (d Date) Format(layout string) string { return d.calendarTime().Format(layout) }
