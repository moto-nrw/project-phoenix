// Package compose wires the People Directory module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// GuardianMembershipQuery maps requested accounts to portal-capable schools.
// The result is reachability evidence, not child-level authorization.
type GuardianMembershipQuery func(context.Context, []int64) (map[int64][]int64, error)

type Dependencies struct {
	StudentOwners              StudentOwners
	StudentClassWriteGateQuery func(context.Context) (*bun.SelectQuery, error)
	DB                         *bun.DB
	Observe                    func(Observation)
	// StudentFieldAudit is the Audit Platform seam behind the per-child
	// change history. Optional: without it the change-history capability
	// reports that it is not configured, which is what graphs that never
	// touch it (CLI roots, repository tests) need.
	StudentFieldAudit StudentFieldAuditLog
	// StudentConsentHistory is the Audit Platform seam behind the shared
	// consent projection. Optional on the same terms; a child without a live
	// photo consent then reports that the trail is not configured rather than
	// rendering a withdrawal as "never granted".
	StudentConsentHistory StudentConsentHistory
	// StudentPhotoRuntime resolves the surfaces the photo lifecycle needs
	// beyond the directory's own rows. It is a resolver, not a value, because
	// the file cleanup, the live refresh and the caller-access gates only
	// exist once the HTTP layer is up. Optional, and a resolver may return
	// nil: every photo route then reports the feature as disabled, which is
	// what graphs that never serve a photo (CLI roots, repository tests)
	// need.
	StudentPhotoRuntime func() StudentPhotoRuntime
	// StudentCompanions is the Care Plan seam the student write path needs:
	// narrowing a departure plan drops the "läuft mit" links it no longer
	// allows. Optional, on the same terms as the others — a graph that never
	// binds it refuses only the writes that would touch a link.
	StudentCompanions StudentCompanions
	// Now is the clock a granted photo consent is stamped with. Optional;
	// time.Now by default.
	Now func() time.Time
}

// New composes People Directory without Identity & Access account projections.
// Graphs that need the parents-app reachability capability use
// NewWithGuardianMemberships so those owner queries stay at the composition seam.
func New(dependencies Dependencies) (*peopledirectory.Module, error) {
	return NewWithGuardianMemberships(dependencies, nil)
}

// NewWithGuardianMemberships composes People Directory with the Identity &
// Access projection that identifies active accounts, guardian roles, and
// active school memberships. It is a constructor argument rather than a
// Dependencies field: only the guardian reachability read needs it, while
// all other People Directory graphs stay independent of Identity & Access.
func NewWithGuardianMemberships(dependencies Dependencies, memberships GuardianMembershipQuery) (*peopledirectory.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("people directory compose: all dependencies are required")
	}
	database := func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("people directory postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("people directory postgres: unsupported transaction %T", transaction)
		}
		return tx, tenant.FromContext(ctx), nil
	}
	observe := func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(postgres.New(database), transaction{}, observe)
	var companions ports.StudentCompanions
	if dependencies.StudentCompanions != nil {
		companions = studentCompanions{seam: dependencies.StudentCompanions}
	}
	owners := studentOwners{owners: dependencies.StudentOwners}
	students := application.NewStudents(postgres.NewStudentStore(database, owners.LockClassWrites, dependencies.StudentClassWriteGateQuery), companions, owners, transaction{}, observe)
	guardians := application.NewGuardians(postgres.NewGuardianStore(database, postgres.PortalMembershipQuery(memberships)), transaction{}, observe)
	var auditLog ports.StudentFieldAuditLog
	if dependencies.StudentFieldAudit != nil {
		auditLog = studentFieldAuditLog{log: dependencies.StudentFieldAudit}
	}
	studentAudit := application.NewStudentAudit(auditLog, observe)
	var consentHistory ports.StudentConsentHistory
	if dependencies.StudentConsentHistory != nil {
		consentHistory = dependencies.StudentConsentHistory
	}
	studentConsents := application.NewStudentConsents(consentHistory, observe)
	photoRuntime := ports.StudentPhotoRuntime(studentPhotoRuntime{resolve: dependencies.StudentPhotoRuntime})
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	studentPhotos := application.NewStudentPhotos(
		postgres.NewStudentStore(database, owners.LockClassWrites, dependencies.StudentClassWriteGateQuery), photoRuntime, transaction{}, observe, now)
	return peopledirectory.NewModule(engine{
		service: service, students: students, guardians: guardians,
		studentAudit: studentAudit, studentConsents: studentConsents,
		studentPhotos: studentPhotos, observe: observe,
	}), nil
}

type transaction struct{}

// TenantID is the tenant the caller's request is scoped to.
func (transaction) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// RegisterAfterCommit defers file cleanup and live refreshes to the commit of
// the transaction the write ran in.
func (transaction) RegisterAfterCommit(ctx context.Context, callback func()) {
	tenant.RegisterAfterCommit(ctx, callback)
}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}

func (transaction) RunSavepoint(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, callback)
}

// RunAdminRead always opens its own admin transaction: a tenant transaction
// already in context would keep row-level security on, which is exactly
// what a platform-wide read must escape.
func (transaction) RunAdminRead(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithinAdmin(tenant.ContextWithoutTransaction(ctx), callback)
}

type engine struct {
	service         *application.Service
	students        *application.StudentService
	guardians       *application.GuardianService
	studentAudit    *application.StudentAuditService
	studentConsents *application.StudentConsentService
	studentPhotos   *application.StudentPhotoService
	observe         func(Observation)
}

func (e engine) Create(ctx context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	value, err := e.service.Create(ctx, domain.CreatePerson{
		FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return toPublic(value), mapError(err)
}

func (e engine) Update(ctx context.Context, input peopledirectory.UpdatePerson) (peopledirectory.Person, error) {
	value, err := e.service.Update(ctx, domain.UpdatePerson{
		ID: input.ID, FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return toPublic(value), mapError(err)
}

func (e engine) Delete(ctx context.Context, id int64) error {
	return mapError(e.service.Delete(ctx, id))
}

func (e engine) FindByID(ctx context.Context, id int64, lock string) (peopledirectory.Person, error) {
	value, err := e.service.FindByID(ctx, id, lock)
	return toPublic(value), mapError(err)
}

func (e engine) FindByAccount(ctx context.Context, accountID int64) (peopledirectory.Person, error) {
	value, err := e.service.FindByAccount(ctx, accountID)
	return toPublic(value), mapError(err)
}

func (e engine) FindByTag(ctx context.Context, tagID string) (peopledirectory.Person, error) {
	value, err := e.service.FindByTag(ctx, tagID)
	return toPublic(value), mapError(err)
}

func (e engine) ListByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func (e engine) ListAcrossTenantsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListAcrossTenantsByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func (e engine) ListByTenantIDs(ctx context.Context, tenantIDs []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByTenantIDs(ctx, tenantIDs)
	return toPublicList(values), mapError(err)
}

func (e engine) ListByAccounts(ctx context.Context, accountIDs []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByAccounts(ctx, accountIDs)
	return toPublicList(values), mapError(err)
}

func (e engine) Search(ctx context.Context, filter peopledirectory.PersonFilter) ([]peopledirectory.Person, error) {
	values, err := e.service.Search(ctx, domain.Filter{
		FirstNamePrefix: filter.FirstNamePrefix, LastNamePrefix: filter.LastNamePrefix,
		FirstNameEquals: filter.FirstNameEquals, LastNameEquals: filter.LastNameEquals,
		FullNameContains: filter.FullNameContains, TagID: filter.TagID, AccountIDs: filter.AccountIDs,
		Page: filter.Page, PageSize: filter.PageSize,
	})
	return toPublicList(values), mapError(err)
}

func (e engine) CountByTenant(ctx context.Context) (map[int64]int, error) {
	value, err := e.service.CountByTenant(ctx)
	return value, mapError(err)
}

func (e engine) LinkAccount(ctx context.Context, personID, accountID int64) error {
	return mapError(e.service.LinkAccount(ctx, personID, accountID))
}

func (e engine) UnlinkAccount(ctx context.Context, personID int64) error {
	return mapError(e.service.UnlinkAccount(ctx, personID))
}

func (e engine) LinkTag(ctx context.Context, personID int64, tagID string) error {
	return mapError(e.service.LinkTag(ctx, personID, tagID))
}

func (e engine) UnlinkTag(ctx context.Context, personID int64) error {
	return mapError(e.service.UnlinkTag(ctx, personID))
}

func (e engine) ReleaseTags(ctx context.Context, personIDs []int64) ([]peopledirectory.ReleasedTag, error) {
	values, err := e.service.ReleaseTags(ctx, personIDs)
	result := make([]peopledirectory.ReleasedTag, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.ReleasedTag{PersonID: value.PersonID, TagID: value.TagID})
	}
	return result, mapError(err)
}

func (e engine) RestoreTag(ctx context.Context, personID int64, tagID string) (bool, error) {
	restored, err := e.service.RestoreTag(ctx, personID, tagID)
	return restored, mapError(err)
}

func toPublic(value domain.Person) peopledirectory.Person {
	return peopledirectory.Person{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
		FirstName: value.FirstName, LastName: value.LastName, Birthday: value.Birthday,
		TagID: value.TagID, AccountID: value.AccountID, DeletedAt: value.DeletedAt,
	}
}

func toPublicList(values []domain.Person) []peopledirectory.Person {
	result := make([]peopledirectory.Person, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return peopledirectory.ErrPersonNotFound
	case errors.Is(err, domain.ErrTagConflict):
		return peopledirectory.ErrTagConflict
	case errors.Is(err, domain.ErrAccountConflict):
		return peopledirectory.ErrAccountConflict
	case errors.Is(err, domain.ErrStudentNotFound):
		return peopledirectory.ErrStudentNotFound
	case errors.Is(err, domain.ErrStudentLockBusy):
		return fmt.Errorf("%w: %w", peopledirectory.ErrStudentLockBusy, err)
	case errors.Is(err, domain.ErrCompanionWouldLoseDeparture):
		return peopledirectory.ErrCompanionWouldLoseDeparture
	case errors.Is(err, domain.ErrCompanionLockBusy):
		return peopledirectory.ErrCompanionLockBusy
	case errors.Is(err, domain.ErrStudentInvalid):
		// The reason is the message a handler renders, so it is kept.
		return &peopledirectory.InvalidStudentError{Reason: err.Error()}
	case errors.Is(err, domain.ErrFamilyProtectionUnchanged):
		return peopledirectory.ErrFamilyProtectionUnchanged
	case errors.Is(err, domain.ErrFamilyProtectionInvalid):
		return peopledirectory.ErrFamilyProtectionInvalid
	default:
		return err
	}
}
