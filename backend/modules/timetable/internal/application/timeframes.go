package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindTimeframe(ctx context.Context, id int64) (result domain.Timeframe, err error) {
	err = s.run("find_timeframe", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindTimeframe(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrTimeframeNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListTimeframes(ctx context.Context, filter domain.TimeframeFilter) (result []domain.Timeframe, err error) {
	err = s.run("list_timeframes", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListTimeframes(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateTimeframe(ctx context.Context, fields domain.TimeframeFields) (result domain.Timeframe, err error) {
	err = s.runWrite(ctx, "create_timeframe", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateTimeframe(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateTimeframe(ctx context.Context, id int64, fields domain.TimeframeFields) (result domain.Timeframe, err error) {
	err = s.runWrite(ctx, "update_timeframe", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateTimeframe(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrTimeframeNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteTimeframe(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_timeframe", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteTimeframe(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}
