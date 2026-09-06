package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) CountStudentAssignments(ctx context.Context, studentID int64) (count int, err error) {
	err = s.run("count_student_assignments", func(stats *domain.OperationStats) error {
		value, queryStats, countErr := s.store.CountStudentAssignments(ctx, studentID)
		stats.Add(queryStats)
		count = value
		return countErr
	})
	return count, err
}

func (s *Service) DeleteStudentAssignments(ctx context.Context, studentID int64) (rows int64, err error) {
	err = s.runWrite(ctx, "delete_student_assignments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, deleteErr := s.store.DeleteStudentAssignments(txCtx, studentID)
		stats.Add(queryStats)
		rows = queryStats.Rows
		return deleteErr
	})
	return rows, err
}
