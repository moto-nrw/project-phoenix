package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NonWorkingDays serves the retained Soll consumers (time tracking, the
// Dienstplan overview) with the union of statutory holidays and closing
// days keyed by calendar date (#1418 3a/3b): a day that is both collapses
// into one key, so it can never be subtracted twice.
type NonWorkingDays struct {
	Calendar schoolcalendar.NonWorkingDayQuery
}

func (d NonWorkingDays) HolidayDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	return calendarDateSet(d.Calendar.NonWorkingDayDates(ctx, from.String(), to.String()))
}

// TenantHolidays serves the consumers that must only see statutory
// holidays (the holidays endpoint, the Feiertagsarbeit warning, the
// statistics reports): closing days never leak in.
type TenantHolidays struct {
	Calendar schoolcalendar.NonWorkingDayQuery
}

func (h TenantHolidays) HolidayDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	return calendarDateSet(h.Calendar.TenantHolidayDates(ctx, from.String(), to.String()))
}

// TenantClosingDays expands the school's closure ranges into a date set.
type TenantClosingDays struct {
	Calendar schoolcalendar.NonWorkingDayQuery
}

func (c TenantClosingDays) ClosingDayDates(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error) {
	return calendarDateSet(c.Calendar.ClosingDayDates(ctx, from.String(), to.String()))
}

func calendarDateSet(values map[string]bool, err error) (map[calendar.Date]bool, error) {
	if err != nil {
		return nil, err
	}
	result := make(map[calendar.Date]bool, len(values))
	for day := range values {
		result[calendar.Date(day)] = true
	}
	return result, nil
}

// CalendarPeriodIDs answers with the ids of every period the tenant still
// has.
type CalendarPeriodIDs struct {
	Calendar schoolcalendar.Query
}

func (p CalendarPeriodIDs) ListCalendarPeriodIDs(ctx context.Context) ([]int64, error) {
	values, err := p.Calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return ids, nil
}

func NewNonWorkingDays(source schoolcalendar.NonWorkingDayQuery) NonWorkingDays {
	return NonWorkingDays{Calendar: source}
}

func NewTenantHolidays(source schoolcalendar.NonWorkingDayQuery) TenantHolidays {
	return TenantHolidays{Calendar: source}
}

func NewTenantClosingDays(source schoolcalendar.NonWorkingDayQuery) TenantClosingDays {
	return TenantClosingDays{Calendar: source}
}

func NewCalendarPeriodIDs(source schoolcalendar.Query) CalendarPeriodIDs {
	return CalendarPeriodIDs{Calendar: source}
}
