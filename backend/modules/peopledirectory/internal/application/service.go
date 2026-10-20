package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// Service owns validation, transactions and the public People Directory capability.
type Service struct {
	students          *StudentService
	guardianRows      *GuardianService
	guardianDirectory peopledirectory.GuardianProvider
	guardianLifecycle *GuardianLifecycle
	studentAudit      *StudentAuditService
	studentConsents   *StudentConsentService
	studentPhotos     *StudentPhotoService
	studentNotes      *StudentNoteService
	store             ports.Store
	tx                ports.Transaction
	observe           ports.Observer
}

func New(store ports.Store, tx ports.Transaction, observe ports.Observer, services Services, guardians ...peopledirectory.GuardianProvider) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("people directory application: all dependencies are required")
	}
	var provider peopledirectory.GuardianProvider
	switch len(guardians) {
	case 0:
	case 1:
		if guardians[0] == nil {
			panic("people directory: guardian provider is required")
		}
		provider = guardians[0]
	default:
		panic("people directory: at most one guardian provider is allowed")
	}
	return &Service{store: store, tx: tx, observe: observe,
		students: services.Students, guardianRows: services.Guardians, guardianDirectory: provider,
		guardianLifecycle: services.GuardianLifecycle, studentAudit: services.StudentAudit,
		studentConsents: services.StudentConsents, studentPhotos: services.StudentPhotos, studentNotes: services.StudentNotes,
	}
}

func (s *Service) Create(ctx context.Context, input peopledirectory.CreatePerson) (result peopledirectory.Person, err error) {
	err = s.runWrite(ctx, "create_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.Create(txCtx, input)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) FindByID(ctx context.Context, id int64, lock string) (result peopledirectory.Person, err error) {
	run := s.runRead
	if lock != "" {
		run = s.runWrite
	}
	err = run(ctx, "find_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, lock)
		stats.Add(queryStats)
		if err == nil && (!found || !peopledirectory.PersonIsLive(result.DeletedAt)) {
			result = peopledirectory.Person{}
			return peopledirectory.ErrPersonNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindByAccount(ctx context.Context, accountID int64) (peopledirectory.Person, error) {
	return s.findOne(ctx, "find_person_by_account", func(txCtx context.Context) (peopledirectory.Person, bool, domain.OperationStats, error) {
		return s.store.FindByAccount(txCtx, accountID)
	})
}

func (s *Service) FindByTag(ctx context.Context, tagID string) (peopledirectory.Person, error) {
	return s.findOne(ctx, "find_person_by_tag", func(txCtx context.Context) (peopledirectory.Person, bool, domain.OperationStats, error) {
		return s.store.FindByTag(txCtx, tagID)
	})
}

func (s *Service) findOne(ctx context.Context, operation string, query func(context.Context) (peopledirectory.Person, bool, domain.OperationStats, error)) (result peopledirectory.Person, err error) {
	err = s.runRead(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = query(txCtx)
		stats.Add(queryStats)
		if err == nil && !found {
			return peopledirectory.ErrPersonNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) requireLocked(ctx context.Context, id int64, stats *domain.OperationStats) error {
	person, found, queryStats, err := s.store.FindByID(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found || !peopledirectory.PersonIsLive(person.DeletedAt) {
		return peopledirectory.ErrPersonNotFound
	}
	return nil
}

func (s *Service) runWrite(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return observeRun(ctx, s.observe, operation, s.tx.RunWrite, fn)
}

func (s *Service) runRead(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return observeRun(ctx, s.observe, operation, s.tx.RunRead, fn)
}

// observeRun runs fn inside run and reports one observation per operation,
// shared by the person and the student service.
func observeRun(ctx context.Context, observe ports.Observer, operation string, run func(context.Context, func(context.Context) error) error, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = run(ctx, func(txCtx context.Context) error { return fn(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	return err
}

// Services are the same-owner operations behind the public capability.
type Services struct {
	Students          *StudentService
	Guardians         *GuardianService
	GuardianLifecycle *GuardianLifecycle
	StudentAudit      *StudentAuditService
	StudentConsents   *StudentConsentService
	StudentPhotos     *StudentPhotoService
	StudentNotes      *StudentNoteService
}

var _ peopledirectory.Owner = (*Service)(nil)

func nonNilPersons(values []peopledirectory.Person) []peopledirectory.Person {
	if values == nil {
		return []peopledirectory.Person{}
	}
	return values
}

func (s *Service) ListAcrossTenantsByIDs(ctx context.Context, ids []int64) (result []peopledirectory.Person, err error) {
	err = observeRun(ctx, s.observe, "list_persons_across_tenants_by_id", s.tx.RunAdminRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// CountByTenant is platform-wide by definition, so it always reads in its
// own admin transaction; a tenant transaction in context would hide every
// other school's rows.
func (s *Service) CountByTenant(ctx context.Context) (result map[int64]int, err error) {
	err = observeRun(ctx, s.observe, "count_persons_by_tenant", s.tx.RunAdminRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountByTenant(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
