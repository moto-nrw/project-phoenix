package application

import (
	"context"

	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CalendarPeriodOverlapListing serves the date-typed overlap finders of the
// legacy calendar period contract over an owner listing keyed by YYYY-MM-DD.
// An empty period type means every type.
type CalendarPeriodOverlapListing struct {
	Source func(ctx context.Context, periodType, start, end string, excludeID int64) ([]*schedule.CalendarPeriod, error)
}

func (l CalendarPeriodOverlapListing) FindActiveOverlapping(ctx context.Context, start, end calendar.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	return l.Source(ctx, "", string(start), string(end), excludeID)
}

func (l CalendarPeriodOverlapListing) FindActiveOverlappingByType(ctx context.Context, periodType string, start, end calendar.Date, excludeID int64) ([]*schedule.CalendarPeriod, error) {
	return l.Source(ctx, periodType, string(start), string(end), excludeID)
}
