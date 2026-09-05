package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindStudentEnrollment(ctx context.Context, id int64) (result domain.StudentEnrollment, err error) {
	err = s.run("find_student_enrollment", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindStudentEnrollment(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrStudentEnrollmentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListStudentEnrollments(ctx context.Context, filter domain.StudentEnrollmentFilter) (result []domain.StudentEnrollment, err error) {
	err = s.run("list_student_enrollments", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListStudentEnrollments(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateStudentEnrollment(ctx context.Context, fields domain.StudentEnrollmentFields) (result domain.StudentEnrollment, err error) {
	err = s.runWrite(ctx, "create_student_enrollment", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateStudentEnrollment(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateStudentEnrollment(ctx context.Context, id int64, fields domain.StudentEnrollmentFields) (result domain.StudentEnrollment, err error) {
	err = s.runWrite(ctx, "update_student_enrollment", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateStudentEnrollment(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrStudentEnrollmentNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteStudentEnrollment(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_student_enrollment", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteStudentEnrollment(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) BackfillStudentEnrollmentSource(ctx context.Context, studentID, requestChildID int64, groupIDs []int64) (result int64, err error) {
	err = s.runWrite(ctx, "backfill_student_enrollment_source", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, writeErr := s.store.BackfillStudentEnrollmentSource(txCtx, studentID, requestChildID, groupIDs)
		stats.Add(queryStats)
		result = rows
		return writeErr
	})
	return result, err
}

func (s *Service) DeleteStudentEnrollmentsBySource(ctx context.Context, studentID, requestChildID int64) (result int64, err error) {
	err = s.runWrite(ctx, "delete_student_enrollments_by_source", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, writeErr := s.store.DeleteStudentEnrollmentsBySource(txCtx, studentID, requestChildID)
		stats.Add(queryStats)
		result = rows
		return writeErr
	})
	return result, err
}

func (s *Service) CapActiveStudentEnrollments(ctx context.Context, groupID int64, validUntil string) (result int64, err error) {
	err = s.runWrite(ctx, "cap_active_student_enrollments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, writeErr := s.store.CapActiveStudentEnrollments(txCtx, groupID, validUntil)
		stats.Add(queryStats)
		result = rows
		return writeErr
	})
	return result, err
}

func (s *Service) SetStudentEnrollmentValidUntil(ctx context.Context, id int64, validUntil string) error {
	return s.runWrite(ctx, "set_student_enrollment_valid_until", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, queryStats, err := s.store.SetStudentEnrollmentValidUntil(txCtx, id, validUntil)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStudentEnrollmentNotFound
		}
		return err
	})
}

func (s *Service) CloseOpenStudentEnrollments(ctx context.Context, groupID int64, periodID *int64, validUntil string) error {
	return s.runWrite(ctx, "close_open_student_enrollments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.CloseOpenStudentEnrollments(txCtx, groupID, periodID, validUntil)
		stats.Add(queryStats)
		return err
	})
}
