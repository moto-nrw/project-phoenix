package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// SetStudentStatus changes one child's lifecycle status. The shared
// class-writes gate comes first, as it does for every other student write.
func (s *StudentService) SetStudentStatus(ctx context.Context, studentID int64, status string) error {
	return s.run(ctx, "set_student_status", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		updated, writeStats, err := s.store.SetStatus(txCtx, studentID, status)
		stats.Add(writeStats)
		if err != nil {
			return err
		}
		if !updated {
			return domain.ErrStudentNotFound
		}
		return nil
	})
}

// TransitionStudentStatus moves a child's status only while the stored one
// still matches expected, reporting false when another writer got there first.
func (s *StudentService) TransitionStudentStatus(
	ctx context.Context,
	studentID int64,
	expected, next string,
) (moved bool, err error) {
	err = s.run(ctx, "transition_student_status", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		var writeStats domain.OperationStats
		moved, writeStats, err = s.store.TransitionStatus(txCtx, studentID, expected, next)
		stats.Add(writeStats)
		return err
	})
	return moved, err
}

// SetStudentCareEnd writes the enrolment interval's upper bound for a batch of
// children and reports how many rows moved.
func (s *StudentService) SetStudentCareEnd(ctx context.Context, ids []int64, until string) (affected int64, err error) {
	if len(ids) == 0 {
		return 0, nil
	}
	err = s.run(ctx, "set_student_care_end", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		var writeStats domain.OperationStats
		affected, writeStats, err = s.store.SetCareEnd(txCtx, ids, until)
		stats.Add(writeStats)
		return err
	})
	return affected, err
}

// ReopenStudentCare gives one child a new start day, clears the end day and
// writes the lifecycle status the caller derived for today.
func (s *StudentService) ReopenStudentCare(ctx context.Context, studentID int64, from, status string) error {
	return s.run(ctx, "reopen_student_care", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		reopened, writeStats, err := s.store.ReopenCare(txCtx, studentID, from, status)
		stats.Add(writeStats)
		if err != nil {
			return err
		}
		if !reopened {
			return domain.ErrStudentNotFound
		}
		return nil
	})
}

// ListStudentCareEnds projects the enrolment upper bound of the given children.
func (s *StudentService) ListStudentCareEnds(ctx context.Context, ids []int64) (bounds map[int64]string, err error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	err = s.run(ctx, "list_student_care_ends", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		bounds, queryStats, err = s.store.ListCareEnds(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return bounds, err
}
