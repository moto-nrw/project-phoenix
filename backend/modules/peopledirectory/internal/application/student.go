package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentService serves the student directory over the same transaction
// and observation seams as the person service.
type StudentService struct {
	owners ports.StudentOwners
	store  ports.StudentStore
	// companions is Care Plan's "läuft mit" edges. Narrowing a child's
	// departure plan has to drop the links it no longer allows, and this is the
	// one write path every writer passes through. Optional: a graph that never
	// binds it refuses only the writes that would touch a link.
	companions ports.StudentCompanions
	tx         ports.Transaction
	observe    ports.Observer
}

func NewStudents(
	store ports.StudentStore,
	companions ports.StudentCompanions,
	owners ports.StudentOwners,
	tx ports.Transaction,
	observe ports.Observer,
) *StudentService {
	if store == nil || tx == nil || observe == nil {
		panic("people directory application: all student dependencies are required")
	}
	return &StudentService{store: store, companions: companions, owners: owners, tx: tx, observe: observe}
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

func (s *StudentService) ListNamesByIDs(ctx context.Context, ids []int64) (result []domain.StudentName, err error) {
	err = s.run(ctx, "list_student_names_by_id", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListNamesByIDs(txCtx, ids)
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

func (s *StudentService) ListByPersonIDs(ctx context.Context, personIDs []int64) (result []domain.Student, err error) {
	err = s.run(ctx, "list_students_by_person", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByPersonIDs(txCtx, personIDs)
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

func (s *StudentService) ListByStatusFlag(ctx context.Context, status string) (result []domain.Student, err error) {
	err = s.run(ctx, "list_students_with_status_flag", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByStatusFlag(txCtx, status)
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

func (s *StudentService) run(ctx context.Context, operation string, run func(context.Context, func(context.Context) error) error, fn func(context.Context, *domain.OperationStats) error) error {
	return observeRun(ctx, s.observe, operation, run, fn)
}
