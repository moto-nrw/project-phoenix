package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindActivityException(ctx context.Context, id int64) (result timetable.ActivityException, err error) {
	if id <= 0 {
		return timetable.ActivityException{}, s.reject("find_activity_exception", timetable.ErrInvalidActivityExceptionQuery)
	}

	err = s.run(ctx, "find_activity_exception", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindActivityException(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrActivityExceptionNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListActivityExceptions(ctx context.Context, filter timetable.ActivityExceptionFilter) (result []timetable.ActivityException, err error) {
	if !validActivityExceptionFilter(filter) {
		return nil, s.reject("list_activity_exceptions", timetable.ErrInvalidActivityExceptionQuery)
	}

	err = s.run(ctx, "list_activity_exceptions", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListActivityExceptions(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountActivityExceptions(ctx context.Context, before *string) (result int, err error) {
	if !validOptionalDate(before) {
		return 0, s.reject("count_activity_exceptions", timetable.ErrInvalidActivityExceptionQuery)
	}

	err = s.run(ctx, "count_activity_exceptions", func(ctx context.Context, stats *domain.OperationStats) error {
		count, queryStats, countErr := s.store.CountActivityExceptions(ctx, before)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

func (s *Service) OldestActivityExceptionBefore(ctx context.Context, before *string) (result *string, err error) {
	if !validOptionalDate(before) {
		return nil, s.reject("oldest_activity_exception", timetable.ErrInvalidActivityExceptionQuery)
	}

	err = s.run(ctx, "oldest_activity_exception", func(ctx context.Context, stats *domain.OperationStats) error {
		value, queryStats, findErr := s.store.OldestActivityExceptionBefore(ctx, before)
		stats.Add(queryStats)
		result = value
		return findErr
	})
	return result, err
}

func (s *Service) CreateActivityException(ctx context.Context, fields timetable.ActivityExceptionInput) (result timetable.ActivityException, err error) {
	if !validActivityException(fields) {
		return timetable.ActivityException{}, s.reject("create_activity_exception", timetable.ErrInvalidActivityException)
	}

	err = s.runWrite(ctx, "create_activity_exception", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateActivityException(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateActivityException(ctx context.Context, id int64, fields timetable.ActivityExceptionInput) (result timetable.ActivityException, err error) {
	if id <= 0 || !validActivityException(fields) {
		return timetable.ActivityException{}, s.reject("update_activity_exception", timetable.ErrInvalidActivityException)
	}

	err = s.runWrite(ctx, "update_activity_exception", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateActivityException(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrActivityExceptionNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteActivityException(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_activity_exception", timetable.ErrInvalidActivityException)
	}

	return s.runWrite(ctx, "delete_activity_exception", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteActivityException(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteActivityExceptionsBefore(ctx context.Context, before string) (result int64, err error) {
	if !timetable.ValidDate(before) {
		return 0, s.reject("delete_activity_exceptions_before", timetable.ErrInvalidActivityException)
	}

	err = s.runWrite(ctx, "delete_activity_exceptions_before", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteActivityExceptionsBefore(txCtx, before)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}
