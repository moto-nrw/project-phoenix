// Package compose wires the People Directory module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type Observation = internalports.Observation

// GuardianLinkOwners are Care Plan's pickup permission and Identity &
// Access's portal access: the two halves a guardian link writes besides its
// relationship (#2756).
type GuardianLinkOwners = internalports.GuardianLinkOwners

// StudentDirectoryRows is the student directory projection the root binds.
type StudentDirectoryRows = postgres.StudentDirectory

// GuardianMembershipQuery maps requested accounts to portal-capable schools.
// The result is reachability evidence, not child-level authorization.
type GuardianMembershipQuery func(context.Context, []int64) (map[int64][]int64, error)

type Dependencies struct {
	StudentOwners              peoplePorts.StudentOwners
	StudentClassWriteGateQuery func(context.Context) (*bun.SelectQuery, error)
	DB                         *bun.DB
	StudentDirectory           StudentDirectoryRows
	Observe                    func(Observation)
	// StudentFieldAudit is the Audit Platform seam behind the per-child
	// change history. Optional: without it the change-history capability
	// reports that it is not configured, which is what graphs that never
	// touch it (CLI roots, repository tests) need.
	StudentFieldAudit StudentFieldAuditLog
	// StudentNoteDeletionAudit records every hidden note in Audit's
	// append-only deletion ledger. It is optional only for compositions that
	// do not serve note deletion; that operation fails closed without it.
	StudentNoteDeletionAudit peoplePorts.StudentNoteDeletionAudit
	// StudentConsentHistory is the Audit Platform seam behind the shared
	// consent projection. Optional on the same terms; a child without a live
	// photo consent then reports that the trail is not configured rather than
	// rendering a withdrawal as "never granted".
	StudentConsentHistory StudentConsentHistory
	// StudentPhotos are the surfaces the photo lifecycle needs beyond the
	// directory's own rows. Optional: without them every photo route reports
	// the feature as disabled, which is what graphs that never serve a photo
	// (CLI roots, repository tests) need.
	StudentPhotos StudentPhotoRuntime
	// StudentCompanions is the Care Plan seam the student write path needs:
	// narrowing a departure plan drops the "läuft mit" links it no longer
	// allows. Optional, on the same terms as the others — a graph that never
	// binds it refuses only the writes that would touch a link.
	StudentCompanions peoplePorts.StudentCompanions
	// GuardianLinkOwners writes the pickup permission and the portal access
	// of a link the parents portal creates or patches. Optional on the same
	// terms: a graph without it refuses only those link writes.
	GuardianLinkOwners GuardianLinkOwners
	// GuardianRolePresets is the role policy the guardian lifecycle links and
	// promotes with. Optional on the same terms.
	GuardianRolePresets peoplePorts.GuardianRolePresets
	// GuardianLinkRows is the guardian link projection the root binds; without
	// it the link reads fail closed.
	GuardianLinkRows postgres.GuardianLinks
	// Now is the clock a granted photo consent is stamped with. Optional;
	// time.Now by default.
	Now func() time.Time
	// GuardianProvider serves the guardian, phone-number and link
	// capability. Optional: without it those flows refuse with
	// peopledirectory.ErrGuardianProviderUnbound.
	GuardianProvider *GuardianDirectoryProvider
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
	if dependencies.DB == nil || dependencies.Observe == nil || dependencies.StudentDirectory == nil {
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
		observation.Err = application.MapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(postgres.New(database), transaction{}, observe)
	var companions internalports.StudentCompanions
	if dependencies.StudentCompanions != nil {
		companions = application.NewStudentCompanions(dependencies.StudentCompanions)
	}
	owners := application.NewStudentOwners(dependencies.StudentOwners)
	noteStore := postgres.NewStudentNoteStore(database)
	var noteDeletionAudit internalports.StudentNoteDeletionAudit
	if dependencies.StudentNoteDeletionAudit != nil {
		noteDeletionAudit = application.NewStudentNoteDeletionAudit(dependencies.StudentNoteDeletionAudit)
	}
	students := application.NewStudents(postgres.NewStudentStore(database, dependencies.StudentDirectory, owners.LockClassWrites, dependencies.StudentClassWriteGateQuery), noteStore, companions, owners, transaction{}, noteDeletionAudit, observe)
	guardians := application.NewGuardians(postgres.NewGuardianStore(database, postgres.PortalMembershipQuery(memberships), dependencies.GuardianLinkRows), dependencies.GuardianLinkOwners, transaction{}, observe)
	var auditLog internalports.StudentFieldAuditLog
	if dependencies.StudentFieldAudit != nil {
		auditLog = application.NewStudentFieldAuditLog(dependencies.StudentFieldAudit)
	}
	studentAudit := application.NewStudentAudit(auditLog, observe)
	studentConsents := application.NewStudentConsents(dependencies.StudentConsentHistory, observe)
	photoRuntime := studentPhotoPort(dependencies.StudentPhotos)
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	studentPhotos := application.NewStudentPhotos(
		postgres.NewStudentStore(database, dependencies.StudentDirectory, owners.LockClassWrites, dependencies.StudentClassWriteGateQuery), photoRuntime, transaction{}, observe, now)
	studentNotes := application.NewStudentNotes(
		noteStore, postgres.New(database),
		postgres.NewStudentStore(database, dependencies.StudentDirectory, owners.LockClassWrites, dependencies.StudentClassWriteGateQuery),
		transaction{}, noteDeletionAudit, observe)
	return newModule(application.NewEngine(application.EngineServices{
		Service: service, Students: students, Guardians: guardians,
		StudentAudit: studentAudit, StudentConsents: studentConsents,
		StudentPhotos: studentPhotos, StudentNotes: studentNotes, Observe: observe,
		GuardianLifecycle: application.NewGuardianLifecycle(guardians, dependencies.GuardianRolePresets),
	}), dependencies.GuardianProvider)
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

// MustNewReads composes the People Directory unobserved and without its write
// owners or audit seams, for the student reads and the row lock other owners'
// compositions take. Composing only fails on a missing dependency, which is a
// programming error at wiring time.
func MustNewReads(db *bun.DB, students StudentDirectoryRows) *peopledirectory.Module {
	module, err := New(Dependencies{DB: db, StudentDirectory: students, Observe: func(Observation) {}})
	if err != nil {
		panic(fmt.Sprintf("compose people directory reads: %v", err))
	}
	return module
}
