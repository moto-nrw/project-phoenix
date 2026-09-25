package timetable

import (
	"context"
	"fmt"
)

// NonWorkingDayCalendar is the Timetable's port onto the school calendar
// (#3594): the statutory holidays of the tenant's federal state and the
// school's own closing days, each as a set of "YYYY-MM-DD" dates in
// [from, to]. The School Calendar owner answers it; Timetable only consumes
// the two date sets and never reads calendar tables itself.
type NonWorkingDayCalendar interface {
	TenantHolidayDates(ctx context.Context, from, to string) (map[string]bool, error)
	ClosingDayDates(ctx context.Context, from, to string) (map[string]bool, error)
}

// OccurrenceSkip names why a series occurrence is not planned on a day.
type OccurrenceSkip int

const (
	// SkipNone plans the occurrence.
	SkipNone OccurrenceSkip = iota
	// SkipHoliday: a statutory holiday. No series runs on it.
	SkipHoliday
	// SkipClosingDay: a closing day of the school. Only a series that opted
	// in with include_closing_days (holiday care) still runs on it.
	SkipClosingDay
)

// NonWorkingDays holds the holidays and closing days of one planning window.
// The zero value skips nothing, which is what a caller without a calendar
// gets.
type NonWorkingDays struct {
	holidays map[string]bool
	closing  map[string]bool
}

// LoadNonWorkingDays reads both date sets of [from, to] once, so a planning
// run can decide every occurrence without further queries. A nil calendar
// yields the zero value.
func LoadNonWorkingDays(ctx context.Context, calendar NonWorkingDayCalendar, from, to string) (NonWorkingDays, error) {
	if calendar == nil {
		return NonWorkingDays{}, nil
	}
	holidays, err := calendar.TenantHolidayDates(ctx, from, to)
	if err != nil {
		return NonWorkingDays{}, fmt.Errorf("load holidays: %w", err)
	}
	closing, err := calendar.ClosingDayDates(ctx, from, to)
	if err != nil {
		return NonWorkingDays{}, fmt.Errorf("load closing days: %w", err)
	}
	return NonWorkingDays{holidays: holidays, closing: closing}, nil
}

// Skip decides whether a series occurrence on date is planned. Holidays win
// over closing days: a holiday inside a closure is skipped even for a series
// that includes closing days.
func (d NonWorkingDays) Skip(date string, includeClosingDays bool) OccurrenceSkip {
	if d.holidays[date] {
		return SkipHoliday
	}
	if d.closing[date] && !includeClosingDays {
		return SkipClosingDay
	}
	return SkipNone
}
