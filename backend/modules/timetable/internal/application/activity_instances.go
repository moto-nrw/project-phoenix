package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindActivityInstance(ctx context.Context, id int64) (result domain.ActivityInstance, err error) {
	err = s.run("find_activity_instance", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindActivityInstance(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrActivityInstanceNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListActivityInstances(ctx context.Context, filter domain.ActivityInstanceFilter) (result []domain.ActivityInstance, err error) {
	err = s.run("list_activity_instances", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListActivityInstances(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) MaxActivityInstanceID(ctx context.Context) (result int64, err error) {
	err = s.run("max_activity_instance_id", func(stats *domain.OperationStats) error {
		value, queryStats, findErr := s.store.MaxActivityInstanceID(ctx)
		stats.Add(queryStats)
		result = value
		return findErr
	})
	return result, err
}

func (s *Service) CountActivityInstances(ctx context.Context, before *string) (result int, err error) {
	err = s.run("count_activity_instances", func(stats *domain.OperationStats) error {
		count, queryStats, countErr := s.store.CountActivityInstances(ctx, before)
		stats.Add(queryStats)
		result = count
		return countErr
	})
	return result, err
}

func (s *Service) OldestActivityInstanceBefore(ctx context.Context, before *string) (result *string, err error) {
	err = s.run("oldest_activity_instance", func(stats *domain.OperationStats) error {
		value, queryStats, findErr := s.store.OldestActivityInstanceBefore(ctx, before)
		stats.Add(queryStats)
		result = value
		return findErr
	})
	return result, err
}

func (s *Service) CreateActivityInstance(ctx context.Context, fields domain.ActivityInstanceFields) (result domain.ActivityInstance, err error) {
	err = s.runWrite(ctx, "create_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateActivityInstance(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) CreateTemplateBackedActivityInstanceIfAbsent(ctx context.Context, fields domain.ActivityInstanceFields) (result domain.ActivityInstance, inserted bool, err error) {
	err = s.runWrite(ctx, "create_template_backed_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, created, queryStats, createErr := s.store.CreateTemplateBackedActivityInstanceIfAbsent(txCtx, fields)
		stats.Add(queryStats)
		result, inserted = value, created
		return createErr
	})
	return result, inserted, err
}

func (s *Service) CreateIdempotentActivityInstance(ctx context.Context, fields domain.ActivityInstanceFields) (result domain.ActivityInstance, inserted bool, err error) {
	err = s.runWrite(ctx, "create_idempotent_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, created, queryStats, createErr := s.store.CreateIdempotentActivityInstance(txCtx, fields)
		stats.Add(queryStats)
		result, inserted = value, created
		return createErr
	})
	return result, inserted, err
}

func (s *Service) UpdateActivityInstance(ctx context.Context, id int64, fields domain.ActivityInstanceFields) (result domain.ActivityInstance, err error) {
	err = s.runWrite(ctx, "update_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateActivityInstance(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrActivityInstanceNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) PatchActivityInstance(ctx context.Context, id int64, fields domain.ActivityInstanceFields, columns []string) (result int64, err error) {
	err = s.runWrite(ctx, "patch_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.PatchActivityInstance(txCtx, id, fields, columns)
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

func (s *Service) DeleteActivityInstance(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_activity_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteActivityInstance(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) MarkActivityInstanceCompleted(ctx context.Context, id int64, completedAt time.Time) error {
	return s.runWrite(ctx, "mark_activity_instance_completed", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.MarkActivityInstanceCompleted(txCtx, id, completedAt)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrActivityInstanceNotFound
		}
		return err
	})
}

func (s *Service) CompleteActiveActivityInstances(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (result int64, err error) {
	err = s.runWrite(ctx, "complete_active_activity_instances", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.CompleteActiveActivityInstances(txCtx, activeGroupIDs, completedAt)
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

func (s *Service) DeletePlannedActivityInstances(ctx context.Context, from string, to *string, groupID *int64, preserveDeviations bool) (result int64, err error) {
	err = s.runWrite(ctx, "delete_planned_activity_instances", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeletePlannedActivityInstances(txCtx, from, to, groupID, preserveDeviations)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

func (s *Service) DeleteRemovedWeekendActivityInstances(ctx context.Context, groupID int64, weekdays []int) (result int64, err error) {
	err = s.runWrite(ctx, "delete_removed_weekend_activity_instances", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteRemovedWeekendActivityInstances(txCtx, groupID, weekdays, s.today())
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}

func (s *Service) PropagateActivityInstanceListKind(ctx context.Context, groupID int64, previousKind, newKind *string, after string) (result int64, err error) {
	err = s.runWrite(ctx, "propagate_activity_instance_list_kind", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.PropagateActivityInstanceListKind(txCtx, groupID, previousKind, newKind, after, time.Now())
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

func (s *Service) DeleteActivityInstancesBefore(ctx context.Context, before string) (result int64, err error) {
	err = s.runWrite(ctx, "delete_activity_instances_before", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteActivityInstancesBefore(txCtx, before)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}
