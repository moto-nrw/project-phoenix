package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
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
	if _, _, err := parseCapabilityRange(from, to); err != nil {
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
	if _, _, err := parseCapabilityRange(from, to); err != nil {
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

// planExportClosingDays and planExportHolidays serve the plan export's
// non-working-day labels from the School Calendar.
type planExportClosingDays struct{ calendar schoolcalendar.Days }

func (a planExportClosingDays) ClosingDaysInRange(ctx context.Context, from, to planexport.Date) ([]*planexport.ClosingPeriod, error) {
	if _, _, err := parseCapabilityRange(string(from), string(to)); err != nil {
		return nil, fmt.Errorf("plan export range: %w", err)
	}
	ranges, err := a.calendar.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: string(from), OverlappingTo: string(to)})
	if err != nil {
		return nil, err
	}
	out := make([]*planexport.ClosingPeriod, 0, len(ranges))
	for _, closing := range ranges {
		out = append(out, &planexport.ClosingPeriod{
			StartDate: planexport.Date(closing.StartDate), EndDate: planexport.Date(closing.EndDate), Reason: closing.Reason,
		})
	}
	return out, nil
}

type planExportHolidays struct{ calendar schoolcalendar.Days }

func (a planExportHolidays) HolidaysInRange(ctx context.Context, from, to planexport.Date) ([]planexport.Holiday, error) {
	if _, _, err := parseCapabilityRange(string(from), string(to)); err != nil {
		return nil, fmt.Errorf("plan export range: %w", err)
	}
	holidays, err := a.calendar.TenantHolidays(ctx, string(from), string(to))
	if err != nil {
		return nil, err
	}
	out := make([]planexport.Holiday, 0, len(holidays))
	for _, holiday := range holidays {
		out = append(out, planexport.Holiday{Date: planexport.Date(holiday.Date), Name: holiday.Name})
	}
	return out, nil
}

// TimeframeChangeGuard binds the schedules API's timeframe edit and delete
// guard: the change is serialized under the tenant recurrence gate and
// refused while a linked care offering still needs the timeframe.
func TimeframeChangeGuard(
	lockRecurrence func(context.Context) error,
	validate func(ctx context.Context, timeframeID int64, replacement *careplan.TimeframeReplacement) error,
) timetableHTTPAdapter.TimeframeChangeGuard {
	return func(ctx context.Context, timeframeID int64, replacement *timetable.TimeframeInput) error {
		if err := lockRecurrence(ctx); err != nil {
			return fmt.Errorf("lock timetable recurrence: %w", err)
		}
		var proposed *careplan.TimeframeReplacement
		if replacement != nil {
			proposed = &careplan.TimeframeReplacement{
				StartTime: replacement.StartTime, EndTime: replacement.EndTime,
				IsActive: replacement.IsActive, Description: replacement.Description,
			}
		}
		if err := validate(ctx, timeframeID, proposed); err != nil {
			if IsCareOfferingConfigInvalid(err) {
				return fmt.Errorf("%w: %w", timetable.ErrTimeframeRequiredByCareOffering, err)
			}
			return fmt.Errorf("validate care offerings: %w", err)
		}
		return nil
	}
}
