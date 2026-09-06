package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindInstanceStaff(ctx context.Context, id int64) (result domain.InstanceStaff, err error) {
	err = s.run("find_instance_staff", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindInstanceStaff(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrInstanceStaffNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListInstanceStaff(ctx context.Context, filter domain.InstanceStaffFilter) (result []domain.InstanceStaff, err error) {
	err = s.run("list_instance_staff", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListInstanceStaff(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CountNonAbsentInstanceStaff(ctx context.Context, instanceIDs []int64) (result map[int64]int, err error) {
	err = s.run("count_non_absent_instance_staff", func(stats *domain.OperationStats) error {
		counts, queryStats, countErr := s.store.CountNonAbsentInstanceStaff(ctx, instanceIDs)
		stats.Add(queryStats)
		result = counts
		return countErr
	})
	return result, err
}

func (s *Service) CreateInstanceStaff(ctx context.Context, fields domain.InstanceStaffFields) (result domain.InstanceStaff, err error) {
	err = s.runWrite(ctx, "create_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateInstanceStaff(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateInstanceStaff(ctx context.Context, id int64, fields domain.InstanceStaffFields) (result domain.InstanceStaff, err error) {
	err = s.runWrite(ctx, "update_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateInstanceStaff(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrInstanceStaffNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) PatchInstanceStaff(ctx context.Context, id int64, fields domain.InstanceStaffFields, columns []string) (result int64, err error) {
	err = s.runWrite(ctx, "patch_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, updateErr := s.store.PatchInstanceStaff(txCtx, id, fields, columns)
		stats.Add(queryStats)
		result = rows
		return updateErr
	})
	return result, err
}

func (s *Service) DeleteInstanceStaff(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStaff(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteInstanceStaffByInstance(ctx context.Context, instanceID int64) error {
	return s.runWrite(ctx, "delete_instance_staff_by_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStaffByInstance(txCtx, instanceID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteUpcomingInstanceStaff(ctx context.Context, staffID int64, after string) (result int64, err error) {
	err = s.runWrite(ctx, "delete_upcoming_instance_staff", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, deleteErr := s.store.DeleteUpcomingInstanceStaff(txCtx, staffID, after)
		stats.Add(queryStats)
		result = rows
		return deleteErr
	})
	return result, err
}
