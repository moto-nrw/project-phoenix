package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	return s.run("lock_instance_student_assignments", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.LockInstanceStudentAssignments(ctx, instanceID)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RestoreInstanceStudentAttendance(ctx context.Context, instanceID int64, rows []domain.CompletionAttendance) error {
	return s.runWrite(ctx, "restore_instance_student_attendance", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		for _, row := range rows {
			queryStats, err := s.store.RestoreInstanceStudentAttendanceRow(txCtx, instanceID, row)
			stats.Add(queryStats)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
