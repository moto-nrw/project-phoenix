package services

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarService "github.com/moto-nrw/project-phoenix/services/calendar"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

type calendarCalDAVSettings interface {
	ResolveBool(context.Context, string) (bool, error)
	ResolveBoolForTenant(context.Context, int64, string) (bool, error)
}

type calendarCalDAVPolicy struct{ settings calendarCalDAVSettings }

func (p calendarCalDAVPolicy) Enabled(ctx context.Context) (bool, error) {
	return p.settings.ResolveBool(ctx, configModels.KeyCalendarCalDAVEnabled)
}

func (p calendarCalDAVPolicy) EnabledForTenant(ctx context.Context, tenantID int64) (bool, error) {
	return p.settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyCalendarCalDAVEnabled)
}

type schoolCalendarHolidayAdapter struct{ query schoolcalendar.HolidayQuery }

func (a schoolCalendarHolidayAdapter) ValidHolidayRegion(region string) bool {
	return a.query.ValidHolidayRegion(region)
}

func (a schoolCalendarHolidayAdapter) ListHolidays(ctx context.Context, region, from, to string) ([]schedule.CalendarHoliday, error) {
	values, err := a.query.ListHolidays(ctx, region, from, to)
	if err != nil {
		return nil, err
	}
	result := make([]schedule.CalendarHoliday, 0, len(values))
	for _, value := range values {
		result = append(result, schedule.CalendarHoliday{Date: value.Date, Name: value.Name})
	}
	return result, nil
}

func (a schoolCalendarHolidayAdapter) HolidayDates(ctx context.Context, region, from, to string) (map[string]bool, error) {
	return a.query.HolidayDates(ctx, region, from, to)
}

type schoolCalendarRendererAdapter struct {
	renderer schoolcalendar.CalendarRenderer
}

func (a schoolCalendarRendererAdapter) RenderCalendar(ctx context.Context, name string, events []calendarService.CalendarEvent) (string, error) {
	values := make([]schoolcalendar.CalendarEvent, 0, len(events))
	for _, event := range events {
		values = append(values, calendarEventToSchoolCalendar(event))
	}
	return a.renderer.RenderCalendar(ctx, name, values)
}

func (a schoolCalendarRendererAdapter) RenderCalendarObject(ctx context.Context, event calendarService.CalendarEvent) (string, error) {
	return a.renderer.RenderCalendarObject(ctx, calendarEventToSchoolCalendar(event))
}

func calendarEventToSchoolCalendar(event calendarService.CalendarEvent) schoolcalendar.CalendarEvent {
	var recurrence *schoolcalendar.CalendarRecurrence
	if event.Recurrence != nil {
		recurrence = &schoolcalendar.CalendarRecurrence{
			Frequency: event.Recurrence.Frequency, Interval: event.Recurrence.Interval,
			Weekdays: event.Recurrence.Weekdays, MonthDays: event.Recurrence.MonthDays,
			Until: event.Recurrence.Until, Count: event.Recurrence.Count,
		}
	}
	return schoolcalendar.CalendarEvent{
		UID: event.UID, Summary: event.Summary, Description: event.Description, Location: event.Location,
		StartDate: event.StartDate, EndDate: event.EndDate, StartClock: event.StartClock, EndClock: event.EndClock,
		AllDay: event.AllDay, Cancelled: event.Cancelled, Sequence: event.Sequence, Stamp: event.Stamp,
		LastModified: event.LastModified, Recurrence: recurrence, ExDates: event.ExDates,
	}
}
