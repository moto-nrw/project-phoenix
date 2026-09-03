package activities

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// CapacityCalendarPeriod is the School Calendar projection the capacity
// query reads. schedule.calendar_periods belongs to that owner (#2666); the
// composition root binds the owner query behind CalendarPeriodSource instead
// of the former SQL subqueries.
type CapacityCalendarPeriod struct {
	ID              int64
	TenantID        int64
	StartDate       timezone.Date
	EndDate         timezone.Date
	WeekCycleLength int
	WeekCycleAnchor *timezone.Date
}

// CalendarPeriodSource is the owner query the activities repositories read
// active calendar periods through. It fails while unbound; there is no
// fallback join.
type CalendarPeriodSource interface {
	// ListActiveCalendarPeriods returns the tenant's active periods ordered
	// by start date.
	ListActiveCalendarPeriods(ctx context.Context) ([]CapacityCalendarPeriod, error)
}

var errCalendarPeriodSourceRequired = errors.New("activities repositories: calendar period source is not bound")

// capacityPeriodColumns splits the periods into the parallel arrays the
// capacity query unnests. Anchors travel as text so an unset anchor stays
// NULL after the NULLIF cast.
func capacityPeriodColumns(periods []CapacityCalendarPeriod) (ids []int64, starts, ends []string, cycleLengths []int64, anchors []string) {
	ids = make([]int64, 0, len(periods))
	starts = make([]string, 0, len(periods))
	ends = make([]string, 0, len(periods))
	cycleLengths = make([]int64, 0, len(periods))
	anchors = make([]string, 0, len(periods))
	for _, period := range periods {
		ids = append(ids, period.ID)
		starts = append(starts, string(period.StartDate))
		ends = append(ends, string(period.EndDate))
		cycleLengths = append(cycleLengths, int64(period.WeekCycleLength))
		anchor := ""
		if period.WeekCycleAnchor != nil {
			anchor = string(*period.WeekCycleAnchor)
		}
		anchors = append(anchors, anchor)
	}
	return ids, starts, ends, cycleLengths, anchors
}
