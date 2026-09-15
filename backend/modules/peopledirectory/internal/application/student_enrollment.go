package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentService) ReadEnrollment(ctx context.Context, id int64, lock string) (result domain.EnrollmentRecord, err error) {
	run := s.tx.RunRead
	if lock != "" {
		run = s.tx.RunWrite
	}
	err = s.run(ctx, "read_enrollment_student", run, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ReadEnrollment(txCtx, id, lock)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) LockEnrollmentClassWrites(ctx context.Context) error {
	return s.run(ctx, "lock_enrollment_class_writes", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(queryStats)
		return err
	})
}

func (s *StudentService) ApplyEnrollmentProfile(ctx context.Context, id int64, input domain.EnrollmentProfilePatch) error {
	return s.run(ctx, "apply_enrollment_profile", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.ApplyEnrollmentProfile(txCtx, id, input)
		stats.Add(queryStats)
		return err
	})
}

func (s *StudentService) CreateEnrollment(ctx context.Context, input domain.EnrollmentStudent) (result domain.Student, err error) {
	err = s.run(ctx, "create_enrollment_student", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateEnrollment(txCtx, input)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) RenewEnrollment(ctx context.Context, id int64, input domain.EnrollmentStudent) error {
	return s.run(ctx, "renew_enrollment_student", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.RenewEnrollment(txCtx, id, input)
		stats.Add(queryStats)
		return err
	})
}
