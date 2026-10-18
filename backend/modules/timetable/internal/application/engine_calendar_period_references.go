package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func (e Engine) CountCalendarPeriodReferences(ctx context.Context) (map[int64]timetable.CalendarPeriodReferences, error) {
	counts, err := e.service.CountCalendarPeriodReferences(ctx)
	if err != nil {
		return nil, MapError(err)
	}
	result := make(map[int64]timetable.CalendarPeriodReferences, len(counts))
	for periodID, entry := range counts {
		result[periodID] = timetable.CalendarPeriodReferences(entry)
	}
	return result, nil
}
