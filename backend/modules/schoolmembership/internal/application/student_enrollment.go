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
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.TransitionStudentStatus(txCtx, id, expected, next)
		stats.Add(queryStats)
		return err
	})
	return changed, err
}

func (s *Service) ResumeStudentCare(ctx context.Context, id int64, from, status, on string) (changed bool, err error) {
	err = s.runWrite(ctx, "resume_student_care", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.ResumeStudentCare(txCtx, id, from, status, on)
		stats.Add(queryStats)
		return err
	})
	return changed, err
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

func (s *Service) ReactivateStudents(ctx context.Context, ids []int64, status string) (changed []int64, err error) {
	err = s.runWrite(ctx, "reactivate_students", func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, gateErr := s.store.LockStudentClassWrites(txCtx, false)
		stats.Add(gateStats)
		if gateErr != nil {
			return gateErr
		}
		var queryStats domain.OperationStats
		changed, queryStats, err = s.store.ReactivateStudents(txCtx, ids, status)
		stats.Add(queryStats)
		return err
	})
	return changed, err
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
