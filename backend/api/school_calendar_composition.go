package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// calendarPeriodReplacement describes the School Calendar's proposed period
// in the catalog's words; nil is the period's removal.
func calendarPeriodReplacement(fields *schoolcalendar.CalendarPeriodFields) *careplan.CalendarPeriodReplacement {
	if fields == nil {
		return nil
	}
	return &careplan.CalendarPeriodReplacement{
		StartDate: calendar.Date(fields.StartDate), EndDate: calendar.Date(fields.EndDate),
		IsActive: fields.IsActive, WeekCycleLength: fields.WeekCycleLength, WeekCycleAnchor: fields.WeekCycleAnchor,
	}
}

// planningCalendarCapability serves workforce.PlanningCalendar from the
// School Calendar: statutory holidays and closing days of the tenant.
type planningCalendarCapability struct{ calendar schoolcalendar.Calendar }

// PlanningCalendarCapability adapts the School Calendar; a nil calendar
// leaves the planning calendar empty.
func PlanningCalendarCapability(calendar schoolcalendar.Calendar) workforce.PlanningCalendar {
	return planningCalendarCapability{calendar: calendar}
}

func (c planningCalendarCapability) HolidaysInRange(ctx context.Context, from, to string) ([]workforce.PublicHoliday, error) {
	if c.calendar == nil {
		return []workforce.PublicHoliday{}, nil
	}
	if _, _, err := owners.ParseCapabilityRange(from, to); err != nil {
		return nil, err
	}
	holidays, err := c.calendar.TenantHolidays(ctx, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.PublicHoliday, 0, len(holidays))
	for _, holiday := range holidays {
		result = append(result, workforce.PublicHoliday{Date: holiday.Date, Name: holiday.Name})
	}
	return result, nil
}

func (c planningCalendarCapability) ClosingDaysInRange(ctx context.Context, from, to string) ([]workforce.ClosingPeriod, error) {
	if c.calendar == nil {
		return []workforce.ClosingPeriod{}, nil
	}
	if _, _, err := owners.ParseCapabilityRange(from, to); err != nil {
		return nil, err
	}
	days, err := c.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: from, OverlappingTo: to})
	if err != nil {
		return nil, err
	}
	result := make([]workforce.ClosingPeriod, 0, len(days))
	for _, day := range days {
		result = append(result, workforce.ClosingPeriod{StartDate: day.StartDate, EndDate: day.EndDate, Reason: day.Reason})
	}
	return result, nil
}
