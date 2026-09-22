package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindInstanceStudent(ctx context.Context, id int64) (result domain.InstanceStudent, err error) {
	err = s.run("find_instance_student", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindInstanceStudent(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrInstanceStudentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListInstanceStudents(ctx context.Context, filter domain.InstanceStudentFilter) (result []domain.InstanceStudent, err error) {
	err = s.run("list_instance_students", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListInstanceStudents(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListPlannedInstanceStudents(ctx context.Context, filter domain.InstanceStudentFilter) (result []domain.PlannedInstanceStudent, err error) {
	err = s.run("list_planned_instance_students", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedInstanceStudents(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateInstanceStudent(ctx context.Context, fields domain.InstanceStudentFields) (result domain.InstanceStudent, err error) {
	err = s.runWrite(ctx, "create_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateInstanceStudent(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) EnsureInstanceStudent(ctx context.Context, instanceID, studentID int64) (result domain.InstanceStudent, inserted bool, err error) {
	err = s.runWrite(ctx, "ensure_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, created, queryStats, ensureErr := s.store.EnsureInstanceStudent(txCtx, instanceID, studentID)
		stats.Add(queryStats)
		result, inserted = value, created
		return ensureErr
	})
	return result, inserted, err
}

func (s *Service) UpdateInstanceStudent(ctx context.Context, id int64, fields domain.InstanceStudentFields) (result domain.InstanceStudent, err error) {
	err = s.runWrite(ctx, "update_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateInstanceStudent(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrInstanceStudentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteInstanceStudent(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStudent(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) error {
	return s.runWrite(ctx, "delete_instance_students_by_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStudentsByInstance(txCtx, instanceID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) (result []domain.StudentInstanceRef, err error) {
	err = s.run("list_student_instance_refs_before", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListStudentInstanceRefsBefore(ctx, cutoff)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) (result []int64, err error) {
	err = s.run("list_planned_student_ids", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedStudentIDs(ctx, studentIDs, date)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	return s.run("lock_instance_student_assignments", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.LockInstanceStudentAssignments(ctx, instanceID)
		stats.Add(queryStats)
		return err
	})
}
