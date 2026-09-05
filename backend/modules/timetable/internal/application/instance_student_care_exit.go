package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) CountPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string, removals []domain.InstanceStudent) (result map[int64]int, err error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	err = s.run("count_planned_student_assignments_after", func(stats *domain.OperationStats) error {
		value, queryStats, operationErr := s.store.CountPlannedStudentAssignmentsAfter(ctx, studentIDs, after, removals)
		stats.Add(queryStats)
		result = value
		return operationErr
	})
	return result, err
}

func (s *Service) RemovePlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) (result []domain.InstanceStudent, err error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	err = s.runWrite(ctx, "remove_planned_student_assignments_after", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, operationErr := s.store.RemovePlannedStudentAssignmentsAfter(txCtx, studentIDs, after)
		stats.Add(queryStats)
		result = value
		return operationErr
	})
	return result, err
}

func (s *Service) RestoreCareExitStudentAssignments(ctx context.Context, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs []int64, removals []domain.InstanceStudent) (result int64, err error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	err = s.runWrite(ctx, "restore_care_exit_student_assignments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, operationErr := s.store.RestoreCareExitStudentAssignments(txCtx, studentIDs, roomIDs, statusDayIDs, pickupExceptionIDs, removals)
		stats.Add(queryStats)
		result = value
		return operationErr
	})
	return result, err
}
