package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/uptrace/bun"
)

// NewSchoolCalendar composes the calendar owner behind the legacy composition
// seam for graphs that do not record observations. Production roots compose
// the module themselves (api/base.go) so runtime evidence is kept. The
// period administration runs without gate, guard and federal state; test
// graphs that bind those use NewSchoolCalendarWithAdministration.
func NewSchoolCalendar(db *bun.DB) (*schoolcalendar.Module, error) {
	return schoolCalendarCompose.New(schoolCalendarCompose.Dependencies{
		DB:      db,
		Observe: func(schoolCalendarCompose.Observation) {},
	})
}

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
