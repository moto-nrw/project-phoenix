package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// CountStudentAssignments includes planned slots and recorded attendance for
// the permanent-deletion preview. Care-exit planning uses a separate query.
func (s *Service) CountStudentAssignments(ctx context.Context, studentID int64) (count int, err error) {
	if studentID <= 0 {
		return 0, s.reject("count_student_assignments", timetable.ErrInvalidInstanceStudentQuery)
	}

	err = s.run(ctx, "count_student_assignments", func(ctx context.Context, stats *domain.OperationStats) error {
		value, queryStats, countErr := s.store.CountStudentAssignments(ctx, studentID)
		stats.Add(queryStats)
		count = value
		return countErr
	})
	return count, err
}

// CountStudentRosterRemovals counts the archived roster rows a grade
// transition parked for the child. They cascade with the student row and are
// reported to the permanent-deletion preview as retained history.
func (s *Service) CountStudentRosterRemovals(ctx context.Context, studentID int64) (count int, err error) {
	if studentID <= 0 {
		return 0, s.reject("count_student_roster_removals", timetable.ErrInvalidInstanceStudentQuery)
	}

	err = s.run(ctx, "count_student_roster_removals", func(ctx context.Context, stats *domain.OperationStats) error {
		value, queryStats, countErr := s.store.CountStudentRosterRemovals(ctx, studentID)
		stats.Add(queryStats)
		count = value
		return countErr
	})
	return count, err
}

// DeleteStudentAssignments removes a child's assignments, never the shared
// instances. The caller's deletion workflow owns the outer transaction.
func (s *Service) DeleteStudentAssignments(ctx context.Context, studentID int64) (rows int64, err error) {
	if studentID <= 0 {
		return 0, s.reject("delete_student_assignments", timetable.ErrInvalidInstanceStudent)
	}

	err = s.runWrite(ctx, "delete_student_assignments", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, deleteErr := s.store.DeleteStudentAssignments(txCtx, studentID)
		stats.Add(queryStats)
		rows = queryStats.Rows
		return deleteErr
	})
	return rows, err
}
