package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) LockOpenStudentAssignments(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	return s.run("lock_open_student_assignments", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.LockOpenStudentAssignments(ctx, studentIDs)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) LockPlannedStudentAssignmentsAfter(ctx context.Context, studentIDs []int64, after string) error {
	if len(studentIDs) == 0 {
		return nil
	}
	return s.run("lock_planned_student_assignments_after", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.LockPlannedStudentAssignmentsAfter(ctx, studentIDs, after)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) ReconnectCareExitAssignmentPickupExceptions(ctx context.Context, studentIDs, pickupExceptionIDs []int64, removals []domain.InstanceStudent) error {
	if len(studentIDs) == 0 {
		return nil
	}
	return s.runWrite(ctx, "reconnect_care_exit_assignment_pickup_exceptions", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.ReconnectCareExitAssignmentPickupExceptions(txCtx, studentIDs, pickupExceptionIDs, removals)
		stats.Add(queryStats)
		return err
	})
}
