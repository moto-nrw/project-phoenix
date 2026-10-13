package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
)

// --- owner queries for the raw repositories ---

// careExitCalendarPeriods answers the booking restore with the ids of every
// period the tenant still has.
type careExitCalendarPeriods struct {
	calendar schoolcalendar.Query
}

func (p careExitCalendarPeriods) ListCalendarPeriodIDs(ctx context.Context) ([]int64, error) {
	values, err := p.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.ID)
	}
	return ids, nil
}
