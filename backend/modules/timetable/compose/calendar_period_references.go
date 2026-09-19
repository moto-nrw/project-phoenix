package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (e engine) CountCalendarPeriodReferences(ctx context.Context) (map[int64]timetable.CalendarPeriodReferences, error) {
	counts, err := e.service.CountCalendarPeriodReferences(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make(map[int64]timetable.CalendarPeriodReferences, len(counts))
	for periodID, entry := range counts {
		result[periodID] = timetable.CalendarPeriodReferences(entry)
	}
	return result, nil
}
