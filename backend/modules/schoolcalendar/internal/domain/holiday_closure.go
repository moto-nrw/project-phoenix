package domain

import (
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayClosureSourceSchoolClosingDay names a school closing day as the
// source of a proposal (ADR 0196).
const HolidayClosureSourceSchoolClosingDay = "school_closing_day"

// HolidayClosureRange is a closing day range.
type HolidayClosureRange struct {
	StartsOn calendar.Date
	EndsOn   calendar.Date
}

// RangeOf reads a closing day range; nil for an empty one.
func RangeOf(start, end string) *HolidayClosureRange {
	if start == "" || end == "" {
		return nil
	}
	return &HolidayClosureRange{StartsOn: calendar.Date(start), EndsOn: calendar.Date(end)}
}

// HolidayClosureChange is one change of a school closing day: its range
// before (nil for a new one) and after (nil for a deletion), under the
// school's revision the change raised. Today is the first day a new
// proposal may still tell.
type HolidayClosureChange struct {
	SourceKind string
	SourceID   int64
	Reason     string
	Previous   *HolidayClosureRange
	Current    *HolidayClosureRange
	Revision   int64
	Today      calendar.Date
}

// Span covers every day the change touches.
func (c HolidayClosureChange) Span() (calendar.Date, calendar.Date) {
	var from, to calendar.Date
	for _, r := range []*HolidayClosureRange{c.Previous, c.Current} {
		if r == nil {
			continue
		}
		if from.IsZero() || r.StartsOn.Before(from) {
			from = r.StartsOn
		}
		if to.IsZero() || r.EndsOn.After(to) {
			to = r.EndsOn
		}
	}
	return from, to
}
