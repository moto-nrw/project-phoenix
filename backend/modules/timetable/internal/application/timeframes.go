package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindTimeframe(ctx context.Context, id int64) (result timetable.Timeframe, err error) {
	if id <= 0 {
		return timetable.Timeframe{}, s.reject("find_timeframe", timetable.ErrInvalidTimeframeQuery)
	}

	err = s.run(ctx, "find_timeframe", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindTimeframe(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrTimeframeNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListTimeframes(ctx context.Context, filter timetable.TimeframeFilter) (result []timetable.Timeframe, err error) {
	if filter.Limit < 0 || filter.Offset < 0 ||
		(filter.OverlapsStart != nil && !validClock(*filter.OverlapsStart)) ||
		(filter.OverlapsEnd != nil && !validClock(*filter.OverlapsEnd)) {
		return nil, s.reject("list_timeframes", timetable.ErrInvalidTimeframeQuery)
	}

	err = s.run(ctx, "list_timeframes", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTimeframes(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateTimeframe(ctx context.Context, fields timetable.TimeframeInput) (result timetable.Timeframe, err error) {
	if !validTimeframe(fields) {
		return timetable.Timeframe{}, s.reject("create_timeframe", timetable.ErrInvalidTimeframe)
	}

	err = s.runWrite(ctx, "create_timeframe", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateTimeframe(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateTimeframe(ctx context.Context, id int64, fields timetable.TimeframeInput) (result timetable.Timeframe, err error) {
	if id <= 0 || !validTimeframe(fields) {
		return timetable.Timeframe{}, s.reject("update_timeframe", timetable.ErrInvalidTimeframe)
	}

	err = s.runWrite(ctx, "update_timeframe", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateTimeframe(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrTimeframeNotFound
		}
		result = value
		return nil
	})
	return result, err
}
