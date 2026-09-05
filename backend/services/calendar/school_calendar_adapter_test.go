package calendar_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarSvc "github.com/moto-nrw/project-phoenix/services/calendar"
)

type testCalendarRenderer struct {
	renderer schoolcalendar.CalendarRenderer
}

func (a testCalendarRenderer) RenderCalendar(ctx context.Context, name string, events []calendarSvc.CalendarEvent) (string, error) {
	values := make([]schoolcalendar.CalendarEvent, 0, len(events))
	for _, event := range events {
		var recurrence *schoolcalendar.CalendarRecurrence
		if event.Recurrence != nil {
			recurrence = &schoolcalendar.CalendarRecurrence{
				Frequency: event.Recurrence.Frequency, Interval: event.Recurrence.Interval,
				Weekdays: event.Recurrence.Weekdays, MonthDays: event.Recurrence.MonthDays,
				Until: event.Recurrence.Until, Count: event.Recurrence.Count,
			}
		}
		values = append(values, schoolcalendar.CalendarEvent{
			UID: event.UID, Summary: event.Summary, Description: event.Description, Location: event.Location,
			StartDate: event.StartDate, EndDate: event.EndDate, StartClock: event.StartClock, EndClock: event.EndClock,
			AllDay: event.AllDay, Cancelled: event.Cancelled, Sequence: event.Sequence, Stamp: event.Stamp,
			LastModified: event.LastModified, Recurrence: recurrence, ExDates: event.ExDates,
		})
	}
	return a.renderer.RenderCalendar(ctx, name, values)
}

func (a testCalendarRenderer) RenderCalendarObject(ctx context.Context, event calendarSvc.CalendarEvent) (string, error) {
	var recurrence *schoolcalendar.CalendarRecurrence
	if event.Recurrence != nil {
		recurrence = &schoolcalendar.CalendarRecurrence{
			Frequency: event.Recurrence.Frequency, Interval: event.Recurrence.Interval,
			Weekdays: event.Recurrence.Weekdays, MonthDays: event.Recurrence.MonthDays,
			Until: event.Recurrence.Until, Count: event.Recurrence.Count,
		}
	}
	return a.renderer.RenderCalendarObject(ctx, schoolcalendar.CalendarEvent{
		UID: event.UID, Summary: event.Summary, Description: event.Description, Location: event.Location,
		StartDate: event.StartDate, EndDate: event.EndDate, StartClock: event.StartClock, EndClock: event.EndClock,
		AllDay: event.AllDay, Cancelled: event.Cancelled, Sequence: event.Sequence, Stamp: event.Stamp,
		LastModified: event.LastModified, Recurrence: recurrence, ExDates: event.ExDates,
	})
}
