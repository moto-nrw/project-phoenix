package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// CountCalendarPeriodReferences reports, per calendar period of the caller's
// school, how many Timetable rows reference it. Periods without references
// are omitted. All Timetable tables are read in one statement (#3124).
func (s *Service) CountCalendarPeriodReferences(ctx context.Context) (result map[int64]timetable.CalendarPeriodReferences, err error) {
	err = s.run(ctx, "count_calendar_period_references", func(ctx context.Context, stats *domain.OperationStats) error {
		counts, queryStats, queryErr := s.store.CountCalendarPeriodReferences(ctx)
		stats.Add(queryStats)
		result = counts
		return queryErr
	})
	return result, err
}
