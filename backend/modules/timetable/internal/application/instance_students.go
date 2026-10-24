package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *Service) FindInstanceStudent(ctx context.Context, id int64) (result timetable.InstanceStudent, err error) {
	if id <= 0 {
		return timetable.InstanceStudent{}, s.reject("find_instance_student", timetable.ErrInvalidInstanceStudentQuery)
	}

	err = s.run(ctx, "find_instance_student", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindInstanceStudent(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrInstanceStudentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error) {
	if !validInstanceStudentFilter(filter) {
		return nil, s.reject("list_instance_students", timetable.ErrInvalidInstanceStudentQuery)
	}
	return s.listInstanceStudents(ctx, filter)
}

// listInstanceStudents reads the rows behind ListInstanceStudents for owner
// reads whose filter the owner built itself.
func (s *Service) listInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) (result []timetable.InstanceStudent, err error) {
	err = s.run(ctx, "list_instance_students", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListInstanceStudents(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListPlannedInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) (result []timetable.PlannedInstanceStudent, err error) {
	if !validInstanceStudentFilter(filter) {
		return nil, s.reject("list_planned_instance_students", timetable.ErrInvalidInstanceStudentQuery)
	}

	err = s.run(ctx, "list_planned_instance_students", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedInstanceStudents(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateInstanceStudent(ctx context.Context, fields timetable.InstanceStudentInput) (result timetable.InstanceStudent, err error) {
	if !validInstanceStudent(fields) {
		return timetable.InstanceStudent{}, s.reject("create_instance_student", timetable.ErrInvalidInstanceStudent)
	}

	err = s.runWrite(ctx, "create_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateInstanceStudent(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) EnsureInstanceStudent(ctx context.Context, instanceID, studentID int64) (result timetable.InstanceStudent, inserted bool, err error) {
	if instanceID <= 0 || studentID <= 0 {
		return timetable.InstanceStudent{}, false, s.reject("ensure_instance_student", timetable.ErrInvalidInstanceStudent)
	}

	err = s.runWrite(ctx, "ensure_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, created, queryStats, ensureErr := s.store.EnsureInstanceStudent(txCtx, instanceID, studentID)
		stats.Add(queryStats)
		result, inserted = value, created
		return ensureErr
	})
	return result, inserted, err
}

func (s *Service) UpdateInstanceStudent(ctx context.Context, id int64, fields timetable.InstanceStudentInput) (result timetable.InstanceStudent, err error) {
	if id <= 0 || !validInstanceStudent(fields) {
		return timetable.InstanceStudent{}, s.reject("update_instance_student", timetable.ErrInvalidInstanceStudent)
	}

	err = s.runWrite(ctx, "update_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateInstanceStudent(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrInstanceStudentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteInstanceStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_instance_student", timetable.ErrInvalidInstanceStudent)
	}

	return s.runWrite(ctx, "delete_instance_student", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStudent(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteInstanceStudentsByInstance(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return s.reject("delete_instance_students_by_instance", timetable.ErrInvalidInstanceStudent)
	}

	return s.runWrite(ctx, "delete_instance_students_by_instance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteInstanceStudentsByInstance(txCtx, instanceID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) ListStudentInstanceRefsBefore(ctx context.Context, cutoff string) (result []timetable.StudentInstanceRef, err error) {
	if !timetable.ValidDate(cutoff) {
		return nil, s.reject("list_student_instance_refs_before", timetable.ErrInvalidInstanceStudentQuery)
	}

	err = s.run(ctx, "list_student_instance_refs_before", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListStudentInstanceRefsBefore(ctx, cutoff)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) ListPlannedStudentIDs(ctx context.Context, studentIDs []int64, date string) (result []int64, err error) {
	if hasInvalidID(studentIDs) || !timetable.ValidDate(date) {
		return nil, s.reject("list_planned_student_ids", timetable.ErrInvalidInstanceStudentQuery)
	}

	err = s.run(ctx, "list_planned_student_ids", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlannedStudentIDs(ctx, studentIDs, date)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	if instanceID <= 0 {
		return s.reject("lock_instance_student_assignments", timetable.ErrInvalidInstanceStudentQuery)
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("timetable: assignment locks require a transaction")
	}

	return s.observeOperation(ctx, "lock_instance_student_assignments", func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockInstanceStudentAssignments(ctx, instanceID)
		stats.Add(queryStats)
		return err
	})
}
