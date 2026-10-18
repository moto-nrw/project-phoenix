// Package compose wires the File Storage module over the shared tenant
// runtime, the Bun database, the object-store backend and the owners it asks
// for facts: Identity & Access for memberships and roles, People Directory
// for the names in the audience picker, the Settings Platform for the two
// file settings, the Audit Platform for the trail, and Communication for the
// announcement side of attachments. Every File Storage table, the cleanup
// intents in documents.file_cleanup included (ADR 0045), is served by the
// owner's own adapter.
package compose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	apiDocuments "github.com/moto-nrw/project-phoenix/modules/filestorage/documents"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Observation is one observed operation of the module.
type Observation = ports.Observation

// ObjectBackend is the object store the module writes bytes to, keyed
// {kind}/{tenant}/{stored name} below the uploads root the root supplies.
type ObjectBackend interface {
	SavePrivate(ctx context.Context, kind string, tenantID int64, storedName string, source io.Reader) (int64, error)
	OpenPrivate(ctx context.Context, kind string, tenantID int64, storedName string) (ports.Object, error)
	RemovePrivate(ctx context.Context, kind string, tenantID int64, storedName string) error
}

const (
	// fileObjectKind is the storage key prefix of the school file storage.
	fileObjectKind = "files"
	// attachmentObjectKind is the prefix of the attachments. It is deliberately
	// distinct: the two have separate tables, intents and audiences, and a
	// shared prefix would let one sweep reach the other's objects.
	attachmentObjectKind = "announcement-attachments"
)

// Identity is the Identity & Access surface the module consumes.
type Identity interface {
	identityaccess.SchoolMembershipQuery
	identityaccess.SchoolRoleQuery
}

// People is the People Directory surface the module consumes.
type People interface {
	ListPersonsByAccount(context.Context, []int64) ([]peopledirectory.Person, error)
}

// Settings is the Settings Platform surface the module consumes.
type Settings = ports.Settings

// Event is one append-only File Storage audit record.
type Event = ports.Event

// Events appends records inside the caller's transaction.
type Events = ports.Events

// Announcements is the Communication surface for the staff side of
// attachments; see ports.Announcements.
type Announcements = ports.Announcements

// GuardianAudience is the parents-portal surface for the guardian side of
// attachments; see ports.GuardianAudience.
type GuardianAudience = ports.GuardianAudience

// Dependencies are the bindings composition supplies. Announcements and
// GuardianAudience are optional: a setup without them serves folders and
// files and refuses every attachment path loudly.
type Dependencies struct {
	DB               *bun.DB
	Objects          ObjectBackend
	Identity         Identity
	People           People
	Settings         Settings
	Events           Events
	HasPermission    func(required string, permissions []string) bool
	Announcements    Announcements
	GuardianAudience GuardianAudience
	Observe          func(Observation)
	Logger           *slog.Logger
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// AttachmentPurgerDependencies are the bindings of the attachment purger:
// the database and the object store alone.
type AttachmentPurgerDependencies struct {
	DB      *bun.DB
	Objects ObjectBackend
	Observe func(Observation)
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// NewAttachmentPurger composes the side of the announcement attachments
// the parent announcements call: cleanup before a deletion, the count for
// the e-mail and the digests of an Erklärung. It needs neither the
// announcements nor the parents portal, so a root hands it to the
// announcements at their construction and builds New over them afterwards
// (#4080). Both are stateless over the same tables and objects.
func NewAttachmentPurger(dependencies AttachmentPurgerDependencies) (filestorage.AttachmentPurger, error) {
	if dependencies.DB == nil || dependencies.Objects == nil || dependencies.Observe == nil {
		return nil, errors.New("file storage compose: all dependencies are required")
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	runtime := databaseRuntime(dependencies.DB)
	return application.NewPublicPurger(application.NewAttachmentPurger(application.AttachmentPurgerDependencies{
		Attachments:        postgres.NewAttachmentStore(runtime),
		AttachmentCleanups: postgres.NewAttachmentCleanupStore(runtime),
		AttachmentObjects:  objectStore{kind: attachmentObjectKind, backend: dependencies.Objects},
		Tx:                 transaction{},
		Observe:            observer(dependencies.Observe),
		Now:                now,
	})), nil
}

// New composes the File Storage module. Every operation resolves the tenant
// from the context; the stores apply it as a defense-in-depth predicate.
func New(dependencies Dependencies) (*filestorage.Module, error) {
	if dependencies.DB == nil || dependencies.Objects == nil || dependencies.Identity == nil ||
		dependencies.People == nil || dependencies.Settings == nil || dependencies.Events == nil ||
		dependencies.Observe == nil || dependencies.Logger == nil || dependencies.HasPermission == nil {
		return nil, errors.New("file storage compose: all dependencies are required")
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	runtime := databaseRuntime(dependencies.DB)
	service := application.New(application.Dependencies{
		Folders:            postgres.NewFolderStore(runtime),
		Files:              postgres.NewFileStore(runtime),
		FileCleanups:       postgres.NewFileCleanupStore(runtime),
		Attachments:        postgres.NewAttachmentStore(runtime),
		AttachmentCleanups: postgres.NewAttachmentCleanupStore(runtime),
		FileObjects:        objectStore{kind: fileObjectKind, backend: dependencies.Objects},
		AttachmentObjects:  objectStore{kind: attachmentObjectKind, backend: dependencies.Objects},
		Identity:           identity{module: dependencies.Identity},
		People:             people{query: dependencies.People},
		Settings:           dependencies.Settings,
		Events:             dependencies.Events,
		Announcements:      dependencies.Announcements,
		GuardianAudience:   dependencies.GuardianAudience,
		Tx:                 transaction{},
		StoredNames:        apiDocuments.NewStoredName,
		Observe:            observer(dependencies.Observe),
		Logger:             dependencies.Logger,
		Now:                now,
	})
	return filestorage.NewModule(application.NewPublicEngine(service, dependencies.HasPermission)), nil
}

// observer reports every observation with the public error.
func observer(observe func(Observation)) func(Observation) {
	return func(observation Observation) {
		observation.Err = application.MapPublicError(observation.Err)
		observe(observation)
	}
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("file storage postgres: tenant is required: %w", err)
		}
		transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			return db, tenantID.Int64(), nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
			return db, tenantID.Int64(), nil
		default:
			return nil, 0, fmt.Errorf("file storage postgres: unsupported transaction %T", transaction)
		}
	}
}

// --- tenant runtime ------------------------------------------------------------

type transaction struct{}

func (transaction) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunInTenant(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, callback)
}

func (transaction) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

func (transaction) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (transaction) Detach(ctx context.Context) context.Context {
	return context.WithoutCancel(tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx)))
}

// --- object store --------------------------------------------------------------

type objectStore struct {
	kind    string
	backend ObjectBackend
}

func (s objectStore) Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error) {
	return s.backend.SavePrivate(ctx, s.kind, tenantID, storedName, source)
}

func (s objectStore) Open(ctx context.Context, tenantID int64, storedName string) (ports.Object, error) {
	return s.backend.OpenPrivate(ctx, s.kind, tenantID, storedName)
}

func (s objectStore) Remove(ctx context.Context, tenantID int64, storedName string) error {
	return s.backend.RemovePrivate(ctx, s.kind, tenantID, storedName)
}

// newStoredName generates the storage name for a validated content type.
// NewDocumentCoordinator constructs the existing object-store adapter.
func NewDocumentCoordinator(kind string, backend apiDocuments.Objects, store apiDocuments.Store, logger *slog.Logger) *apiDocuments.Coordinator {
	return &apiDocuments.Coordinator{Kind: kind, Backend: backend, Store: store, Logger: logger}
}

// --- foreign facts -------------------------------------------------------------

type identity struct{ module Identity }

func (i identity) HasActiveMembership(ctx context.Context, accountID int64) (bool, error) {
	return i.module.HasActiveSchoolMembership(ctx, accountID)
}

func (i identity) ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return i.module.ListAccountRoleIDs(ctx, accountID)
}

// ListShareableRoles drops the guardian tier: parents never reach the tenant
// portal, so a folder shared with them would be shared with nobody.
func (i identity) ListShareableRoles(ctx context.Context) ([]domain.AudienceRole, error) {
	roles, err := i.module.ListSchoolRoles(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AudienceRole, 0, len(roles))
	for _, role := range roles {
		if role == nil || role.Name == "guardian" || (role.BaseRole != nil && *role.BaseRole == "guardian") {
			continue
		}
		result = append(result, domain.AudienceRole{ID: role.ID, Name: role.Name})
	}
	return result, nil
}

func (i identity) ListActiveAccountIDs(ctx context.Context) ([]int64, error) {
	return i.module.ListActiveSchoolAccountIDs(ctx)
}

type people struct{ query People }

func (p people) PersonNames(ctx context.Context, accountIDs []int64) (map[int64]ports.PersonName, error) {
	persons, err := p.query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	newest := make(map[int64]peopledirectory.Person, len(persons))
	for _, person := range persons {
		if person.AccountID == nil {
			continue
		}
		current, found := newest[*person.AccountID]
		if !found || person.UpdatedAt.After(current.UpdatedAt) {
			newest[*person.AccountID] = person
		}
	}
	result := make(map[int64]ports.PersonName, len(newest))
	for accountID, person := range newest {
		result[accountID] = ports.PersonName{FirstName: person.FirstName, LastName: person.LastName}
	}
	return result, nil
}
