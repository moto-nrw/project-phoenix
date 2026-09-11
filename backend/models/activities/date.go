package activities

import (
	"encoding/json"
	"fmt"
	"time"
)

const dateLayout = "2006-01-02"

// Date is the retained activity-model representation of a calendar day.
// Owner-facing Timetable contracts use YYYY-MM-DD strings.
type Date string

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
	parsed, err := time.Parse(dateLayout, string(d))
	if err != nil {
		return ""
	}
	return Date(parsed.AddDate(0, 0, days).Format(dateLayout))
}

func (d Date) MarshalText() ([]byte, error) { return []byte(d), nil }

func (d *Date) UnmarshalText(value []byte) error {
	parsed, err := time.Parse(dateLayout, string(value))
	if err != nil || parsed.Format(dateLayout) != string(value) {
		return fmt.Errorf("invalid calendar date %q", value)
	}
	*d = Date(value)
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
		return fmt.Errorf("activities.Date: %w", err)
	}
	return d.UnmarshalText([]byte(text))
}
