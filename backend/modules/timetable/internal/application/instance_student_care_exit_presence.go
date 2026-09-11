package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) ListOpenStudentAssignments(ctx context.Context, studentIDs []int64) (result []int64, err error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	err = s.run("list_open_student_assignments", func(stats *domain.OperationStats) error {
		value, queryStats, operationErr := s.store.ListOpenStudentAssignments(ctx, studentIDs)
		stats.Add(queryStats)
		result = value
		return operationErr
	})
	return result, err
}

func (s *Service) LatestStudentAssignmentAttendanceDate(ctx context.Context, studentID int64) (result *string, err error) {
	err = s.run("latest_student_assignment_attendance_date", func(stats *domain.OperationStats) error {
		value, queryStats, operationErr := s.store.LatestStudentAssignmentAttendanceDate(ctx, studentID)
		stats.Add(queryStats)
		result = value
		return operationErr
	})
	return result, err
}

func (s *Service) CloseOpenStudentAssignments(ctx context.Context, studentIDs []int64, at time.Time) (result int64, err error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "close_open_student_assignments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, operationErr := s.store.CloseOpenStudentAssignments(txCtx, studentIDs, at)
		stats.Add(queryStats)
		result = value
		return operationErr
	})
	return result, err
}
