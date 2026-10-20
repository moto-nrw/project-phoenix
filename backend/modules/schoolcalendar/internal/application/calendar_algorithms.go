package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// CalendarAlgorithms computes statutory holidays and renders RFC5545 documents.
type CalendarAlgorithms interface {
	ValidHolidayRegion(string) bool
	ListHolidays(context.Context, string, string, string) ([]schoolcalendar.Holiday, error)
	HolidayDates(context.Context, string, string, string) (map[string]bool, error)
	RenderCalendar(context.Context, string, []schoolcalendar.CalendarEvent) (string, error)
	RenderCalendarObject(context.Context, schoolcalendar.CalendarEvent) (string, error)
}

// LocalCalendar computes and renders locally, without network access.
type LocalCalendar struct{}

func (LocalCalendar) ValidHolidayRegion(region string) bool { return ValidHolidayRegion(region) }
func (LocalCalendar) ListHolidays(_ context.Context, region, from, to string) ([]schoolcalendar.Holiday, error) {
	return HolidaysInRange(region, from, to)
}
func (LocalCalendar) HolidayDates(_ context.Context, region, from, to string) (map[string]bool, error) {
	return HolidayDates(region, from, to)
}
func (LocalCalendar) RenderCalendar(_ context.Context, name string, events []schoolcalendar.CalendarEvent) (string, error) {
	return RenderCalendar(name, events), nil
}
func (LocalCalendar) RenderCalendarObject(_ context.Context, event schoolcalendar.CalendarEvent) (string, error) {
	return RenderCalendarObject(event), nil
}
