package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Service) ListClassAssignments(ctx context.Context, filter domain.ClassAssignmentFilter) (result []domain.ClassAssignment, err error) {
	err = s.runRead(ctx, "list_class_assignments", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClassAssignments(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateClassAssignment(ctx context.Context, staffID int64, schoolClass string) (result domain.ClassAssignment, err error) {
	err = s.runWrite(ctx, "create_class_assignment", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireStaffLocked(txCtx, staffID, stats); err != nil {
			return err
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateClassAssignment(txCtx, staffID, schoolClass)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateClassAssignment(ctx context.Context, id, staffID int64, schoolClass string) (result domain.ClassAssignment, err error) {
	err = s.runWrite(ctx, "update_class_assignment", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireStaffLocked(txCtx, staffID, stats); err != nil {
			return err
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UpdateClassAssignment(txCtx, id, staffID, schoolClass)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteClassAssignment(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_class_assignment", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteClassAssignment(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteClassAssignmentsByStaff(ctx context.Context, staffID int64) (rows int64, err error) {
	err = s.runWrite(ctx, "delete_class_assignments_by_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, deleteErr := s.store.DeleteClassAssignmentsByStaff(txCtx, staffID)
		stats.Add(queryStats)
		rows = queryStats.Rows
		return deleteErr
	})
	return rows, err
}

func (s *Service) ListGroupAssignments(ctx context.Context, filter domain.GroupAssignmentFilter) (result []domain.GroupAssignment, err error) {
	err = s.runRead(ctx, "list_group_assignments", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGroupAssignments(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateGroupAssignment(ctx context.Context, groupID, teacherID int64) (result domain.GroupAssignment, err error) {
	err = s.runWrite(ctx, "create_group_assignment", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireTeacherForAssignment(txCtx, teacherID, stats); err != nil {
			return err
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateGroupAssignment(txCtx, groupID, teacherID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateGroupAssignment(ctx context.Context, id, groupID, teacherID int64) (result domain.GroupAssignment, err error) {
	err = s.runWrite(ctx, "update_group_assignment", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireTeacherForAssignment(txCtx, teacherID, stats); err != nil {
			return err
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UpdateGroupAssignment(txCtx, id, groupID, teacherID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteGroupAssignment(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_group_assignment", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteGroupAssignment(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteGroupAssignmentsByTeacher(ctx context.Context, teacherID int64) (rows int64, err error) {
	return s.deleteGroupAssignments(ctx, "delete_group_assignments_by_teacher", func(txCtx context.Context) (domain.OperationStats, error) {
		return s.store.DeleteGroupAssignmentsByTeacher(txCtx, teacherID)
	})
}

func (s *Service) deleteGroupAssignments(ctx context.Context, operation string, remove func(context.Context) (domain.OperationStats, error)) (rows int64, err error) {
	err = s.runWrite(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, deleteErr := remove(txCtx)
		stats.Add(queryStats)
		rows = queryStats.Rows
		return deleteErr
	})
	return rows, err
}

// Assignment writers and retirement serialize on the teacher row. A foreign
// key alone cannot reject the retained tombstone after retirement commits.
func (s *Service) requireTeacherForAssignment(ctx context.Context, teacherID int64, stats *domain.OperationStats) error {
	_, found, queryStats, err := s.store.FindTeacher(ctx, teacherID, "SHARE")
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrTeacherNotFound
	}
	return nil
}
