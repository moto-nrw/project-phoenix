package compose

import "context"

func (e engine) CountPlannedSupervisorsByCalendarPeriod(ctx context.Context) (map[int64]int, error) {
	counts, err := e.service.CountPlannedSupervisorsByCalendarPeriod(ctx)
	return counts, mapError(err)
}
