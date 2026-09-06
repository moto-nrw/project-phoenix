package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) CountPlannedSupervisorsByCalendarPeriod(ctx context.Context) (result map[int64]int, err error) {
	err = s.run("count_planned_supervisors_by_calendar_period", func(stats *domain.OperationStats) error {
		counts, queryStats, queryErr := s.store.CountPlannedSupervisorsByCalendarPeriod(ctx)
		stats.Add(queryStats)
		result = counts
		return queryErr
	})
	return result, err
}
