package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

func (s *Service) TransitionStudentStatus(ctx context.Context, id int64, expected, next string) (changed bool, err error) {
	err = s.runWrite(ctx, "transition_student_status", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		transition := func(writeCtx context.Context) (bool, error) {
			moved, queryStats, writeErr := s.store.TransitionStudentStatus(writeCtx, id, expected, next)
			stats.Add(queryStats)
			changed = moved
			return moved, writeErr
		}
		if expected == "pending" && next == "active" {
			return s.countingTransition(txCtx, stats, transition)
		}
		_, err := transition(txCtx)
		return err
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

// SetStudentStatus is the unconditional status change. Unlike the
// scheduler's compare-and-set it can bring an inactive child back, so it is
// checked against the Kinderkontingent (#3567).
func (s *Service) SetStudentStatus(ctx context.Context, id int64, status string) (changed bool, err error) {
	err = s.runWrite(ctx, "set_student_status", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		return s.countingWrite(txCtx, stats, func(writeCtx context.Context) error {
			moved, queryStats, writeErr := s.store.TransitionStudentStatus(writeCtx, id, "", status)
			stats.Add(queryStats)
			changed = moved
			return writeErr
		})
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (s *Service) ResumeStudentCare(ctx context.Context, id int64, from, status, on string) (changed bool, err error) {
	err = s.runWrite(ctx, "resume_student_care", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		return s.countingWrite(txCtx, stats, func(writeCtx context.Context) error {
			resumed, queryStats, writeErr := s.store.ResumeStudentCare(writeCtx, id, from, status, on)
			stats.Add(queryStats)
			changed = resumed
			return writeErr
		})
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}

func (s *Service) EndStudentCare(ctx context.Context, ids []int64, until string) (changed int64, err error) {
	err = s.runWrite(ctx, "end_student_care", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.EndStudentCare(txCtx, ids, until)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

// ReactivateStudents brings graduates back. Only the grade-transition revert
// passes enforceChildQuota=false (#3567).
func (s *Service) ReactivateStudents(ctx context.Context, ids []int64, status string, enforceChildQuota bool) (changed []int64, err error) {
	err = s.runWrite(ctx, "reactivate_students", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		reactivate := func(writeCtx context.Context) error {
			reactivated, queryStats, writeErr := s.store.ReactivateStudents(writeCtx, ids, status)
			stats.Add(queryStats)
			changed = reactivated
			return writeErr
		}
		if !enforceChildQuota {
			return reactivate(txCtx)
		}
		return s.countingWrite(txCtx, stats, reactivate)
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

func (s *Service) GraduateStudents(ctx context.Context, ids []int64) (changed int64, err error) {
	err = s.runWrite(ctx, "graduate_students", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.GraduateStudents(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

func (s *Service) ChangeStudentClass(ctx context.Context, ids []int64, from, to string) (changed int64, err error) {
	err = s.runWrite(ctx, "change_student_class", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.ChangeStudentClass(txCtx, ids, from, to)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

func (s *Service) LockStudentClassWrites(ctx context.Context, exclusive bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := s.runWrite(ctx, "lock_student_class_writes", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockStudentClassWrites(txCtx, exclusive)
		stats.Add(queryStats)
		return err
	})

	if err != nil {
		return fmt.Errorf("school membership: lock class writes: %w", err)
	}
	return nil
}
