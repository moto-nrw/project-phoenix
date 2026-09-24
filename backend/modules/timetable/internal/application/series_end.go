package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// dropOccurrencesAfterSeriesEnd removes the future occurrences a shortened
// series no longer has (#3594). The materializer never plans past the last
// day, but rows planned before the edit would otherwise stay, whatever window
// a later re-plan covers. Blocks that started and occurrences with individual
// staff changes or an understaffed acknowledgement keep their row.
func (s *Service) dropOccurrencesAfterSeriesEnd(ctx context.Context, groupID int64, fields domain.TemplateFields, stats *domain.OperationStats) error {
	if !fields.SeriesLastDayProvided || fields.SeriesLastDay == nil {
		return nil
	}
	lastDay, err := calendar.ParseDate(*fields.SeriesLastDay)
	if err != nil {
		return err
	}
	from := lastDay.AddDays(1).String()
	if today := s.today(); today > from {
		from = today
	}
	ids, queryStats, err := s.store.ListReplannableActivityInstanceIDs(ctx, from, nil, &groupID, true)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	var removed int64
	return s.deleteUnstartedInstances(ctx, ids, stats, &removed)
}
