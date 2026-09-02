package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentService serves the student directory over the same transaction
// and observation seams as the person service.
type StudentService struct {
	store   ports.StudentStore
	tx      ports.Transaction
	observe ports.Observer
}

func NewStudents(store ports.StudentStore, tx ports.Transaction, observe ports.Observer) *StudentService {
	if store == nil || tx == nil || observe == nil {
		panic("people directory application: all student dependencies are required")
	}
	return &StudentService{store: store, tx: tx, observe: observe}
}

func (s *StudentService) ListByIDs(ctx context.Context, ids []int64) (result []domain.Student, err error) {
	err = s.run(ctx, "list_students_by_id", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// ListAcrossTenantsByIDs reads in a separate admin transaction so a hosting
// tenant's row-level security does not hide visiting students.
func (s *StudentService) ListAcrossTenantsByIDs(ctx context.Context, ids []int64) (result []domain.Student, err error) {
	err = s.run(ctx, "list_students_across_tenants_by_id", s.tx.RunAdminRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) ListByClasses(ctx context.Context, classes []string) (result []domain.Student, err error) {
	err = s.run(ctx, "list_students_by_class", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByClasses(txCtx, classes)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) ListEnrolled(ctx context.Context) (result []domain.Student, err error) {
	err = s.run(ctx, "list_enrolled_students", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListEnrolled(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) ListClasses(ctx context.Context) (result []string, err error) {
	err = s.run(ctx, "list_school_classes", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClasses(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) Lock(ctx context.Context, id int64) error {
	return s.run(ctx, "lock_student", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		found, lockStats, err := s.store.Lock(txCtx, id)
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStudentNotFound
		}
		return nil
	})
}

func (s *StudentService) Promote(ctx context.Context, ids []int64, fromClass, toClass string) (affected int64, err error) {
	err = s.run(ctx, "promote_students", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		affected, writeStats, err = s.store.Promote(txCtx, ids, fromClass, toClass)
		stats.Add(writeStats)
		return err
	})
	return affected, err
}

func (s *StudentService) RevertClass(ctx context.Context, id int64, fromClass, toClass string) (affected int64, err error) {
	err = s.run(ctx, "revert_student_class", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		affected, writeStats, err = s.store.RevertClass(txCtx, id, fromClass, toClass)
		stats.Add(writeStats)
		return err
	})
	return affected, err
}

func (s *StudentService) GraduateByClasses(ctx context.Context, classes []string) (affected int64, err error) {
	err = s.run(ctx, "graduate_students_by_class", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		affected, writeStats, err = s.store.GraduateByClasses(txCtx, classes)
		stats.Add(writeStats)
		return err
	})
	return affected, err
}

func (s *StudentService) GraduateByIDs(ctx context.Context, ids []int64) (affected int64, err error) {
	err = s.run(ctx, "graduate_students", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		affected, writeStats, err = s.store.GraduateByIDs(txCtx, ids)
		stats.Add(writeStats)
		return err
	})
	return affected, err
}

func (s *StudentService) Reactivate(ctx context.Context, ids []int64, status string) (result []int64, err error) {
	err = s.run(ctx, "reactivate_students", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var writeStats domain.OperationStats
		result, writeStats, err = s.store.Reactivate(txCtx, ids, status)
		stats.Add(writeStats)
		return err
	})
	if result == nil {
		result = []int64{}
	}
	return result, err
}

func (s *StudentService) run(ctx context.Context, operation string, run func(context.Context, func(context.Context) error) error, fn func(context.Context, *domain.OperationStats) error) error {
	return observeRun(ctx, s.observe, operation, run, fn)
}
