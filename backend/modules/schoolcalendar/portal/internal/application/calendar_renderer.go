package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
)

// NewCalendarRenderer serves the portal's iCalendar renderer port from the
// School Calendar's renderer.
func NewCalendarRenderer(renderer schoolcalendar.CalendarRenderer) portal.CalendarRenderer {
	return calendarRenderer{renderer: renderer}
}

type calendarRenderer struct {
	renderer schoolcalendar.CalendarRenderer
}

func (a calendarRenderer) RenderCalendar(ctx context.Context, name string, events []portal.CalendarEvent) (string, error) {
	values := make([]schoolcalendar.CalendarEvent, 0, len(events))
	for _, event := range events {
		values = append(values, schoolCalendarEvent(event))
	}
	return a.renderer.RenderCalendar(ctx, name, values)
}

func (a calendarRenderer) RenderCalendarObject(ctx context.Context, event portal.CalendarEvent) (string, error) {
	return a.renderer.RenderCalendarObject(ctx, schoolCalendarEvent(event))
}

func schoolCalendarEvent(event portal.CalendarEvent) schoolcalendar.CalendarEvent {
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
