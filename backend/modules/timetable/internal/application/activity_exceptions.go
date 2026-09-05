package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindActivityException(ctx context.Context, id int64) (result domain.ActivityException, err error) {
	err = s.run("find_activity_exception", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindActivityException(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrActivityExceptionNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListActivityExceptions(ctx context.Context, filter domain.ActivityExceptionFilter) (result []domain.ActivityException, err error) {
	err = s.run("list_activity_exceptions", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListActivityExceptions(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountActivityExceptions(ctx context.Context, before *string) (result int, err error) {
	err = s.run("count_activity_exceptions", func(stats *domain.OperationStats) error {
		count, queryStats, countErr := s.store.CountActivityExceptions(ctx, before)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

func (s *Service) OldestActivityExceptionBefore(ctx context.Context, before *string) (result *string, err error) {
	err = s.run("oldest_activity_exception", func(stats *domain.OperationStats) error {
		value, queryStats, findErr := s.store.OldestActivityExceptionBefore(ctx, before)
		stats.Add(queryStats)
		result = value
		return findErr
	})
	return result, err
}

func (s *Service) CreateActivityException(ctx context.Context, fields domain.ActivityExceptionFields) (result domain.ActivityException, err error) {
	err = s.runWrite(ctx, "create_activity_exception", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateActivityException(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateActivityException(ctx context.Context, id int64, fields domain.ActivityExceptionFields) (result domain.ActivityException, err error) {
	err = s.runWrite(ctx, "update_activity_exception", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateActivityException(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrActivityExceptionNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteActivityException(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_activity_exception", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteActivityException(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteActivityExceptionsBefore(ctx context.Context, before string) (result int64, err error) {
	err = s.runWrite(ctx, "delete_activity_exceptions_before", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteActivityExceptionsBefore(txCtx, before)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}
