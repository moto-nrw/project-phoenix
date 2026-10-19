package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) CountCalendarPeriodReferences(ctx context.Context) (result map[int64]domain.CalendarPeriodReferences, err error) {
	err = s.run(ctx, "count_calendar_period_references", func(ctx context.Context, stats *domain.OperationStats) error {
		counts, queryStats, queryErr := s.store.CountCalendarPeriodReferences(ctx)
		stats.Add(queryStats)
		result = counts
		return queryErr
	})
	return result, err
}
