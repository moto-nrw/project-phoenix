//go:build testhooks

package application

import (
	"context"

	schedule "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClosingDayRangeListing binds the legacy date-typed range finder to a YYYY-MM-DD listing.
type ClosingDayRangeListing struct {
	Source func(ctx context.Context, from, to string) ([]*schedule.ClosingDay, error)
}

func (l ClosingDayRangeListing) FindOverlappingRange(ctx context.Context, from, to calendar.Date) ([]*schedule.ClosingDay, error) {
	return l.Source(ctx, string(from), string(to))
}
