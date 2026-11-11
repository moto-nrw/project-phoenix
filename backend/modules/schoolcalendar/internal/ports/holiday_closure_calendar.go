package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayClosureCalendarReads reads and locks current schools in one batch.
type HolidayClosureCalendarReads interface {
	HolidayClosureCalendars(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date) (map[int64]schoolcalendar.HolidayClosureCalendar, error)
}

// HolidayClosureHolidays computes statutory holidays without a database read.
type HolidayClosureHolidays interface {
	ValidHolidayRegion(string) bool
	ListHolidays(ctx context.Context, region, from, to string) ([]schoolcalendar.Holiday, error)
}
