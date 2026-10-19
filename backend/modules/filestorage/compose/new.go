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
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	apiDocuments "github.com/moto-nrw/project-phoenix/modules/filestorage/internal/adapters/documents"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Observation is one observed operation of the module.
type Observation = ports.Observation

// ObjectBackend is the object store the module writes bytes to, keyed
// {kind}/{tenant}/{stored name} below the uploads root the root supplies.
type ObjectBackend interface {
	application.ObjectBackend
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
type Identity interface{ application.Identity }

// People is the People Directory surface the module consumes.
type People interface{ application.People }

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
	runtime := postgres.DatabaseRuntime(dependencies.DB)
	return application.NewPublicPurger(application.NewAttachmentPurger(application.AttachmentPurgerDependencies{
		Attachments:        postgres.NewAttachmentStore(runtime),
		AttachmentCleanups: postgres.NewAttachmentCleanupStore(runtime),
		AttachmentObjects:  application.ObjectStore{Kind: attachmentObjectKind, Backend: dependencies.Objects},
		Tx:                 transaction{},
		Observe:            application.Observer(dependencies.Observe),
		Now:                dependencies.Now,
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
	runtime := postgres.DatabaseRuntime(dependencies.DB)
	service := application.New(application.Dependencies{
		Folders:            postgres.NewFolderStore(runtime),
		Files:              postgres.NewFileStore(runtime),
		FileCleanups:       postgres.NewFileCleanupStore(runtime),
		Attachments:        postgres.NewAttachmentStore(runtime),
		AttachmentCleanups: postgres.NewAttachmentCleanupStore(runtime),
		FileObjects:        application.ObjectStore{Kind: fileObjectKind, Backend: dependencies.Objects},
		AttachmentObjects:  application.ObjectStore{Kind: attachmentObjectKind, Backend: dependencies.Objects},
		Identity:           application.IdentityAdapter{Module: dependencies.Identity},
		People:             application.PeopleAdapter{Query: dependencies.People},
		Settings:           dependencies.Settings,
		Events:             dependencies.Events,
		Announcements:      dependencies.Announcements,
		GuardianAudience:   dependencies.GuardianAudience,
		Tx:                 transaction{},
		StoredNames:        apiDocuments.NewStoredName,
		Observe:            application.Observer(dependencies.Observe),
		Logger:             dependencies.Logger,
		Now:                dependencies.Now,
	})
	return filestorage.NewModule(application.NewPublicEngine(service, dependencies.HasPermission)), nil
}

// --- tenant runtime ------------------------------------------------------------

// --- object store --------------------------------------------------------------

// newStoredName generates the storage name for a validated content type.
// NewDocumentCoordinator constructs the existing object-store adapter.
func NewDocumentCoordinator(kind string, backend apiDocuments.Objects, store apiDocuments.Store, logger *slog.Logger) *apiDocuments.Coordinator {
	return &apiDocuments.Coordinator{Kind: kind, Backend: backend, Store: store, Logger: logger}
}

// --- foreign facts -------------------------------------------------------------

// NewUploadCleanup uses the existing file cleanup outbox, partitioned by kind.
func NewUploadCleanup(db *bun.DB, kind filestorage.UploadCleanupKind) *filestorage.UploadCleanup {
	cleanup := application.NewUploadCleanup(postgres.NewUploadCleanupStore(postgres.DatabaseRuntime(db), string(kind)), transaction{}, string(kind))
	return filestorage.NewUploadCleanup(cleanup.Queue, cleanup.Sweep)
}

type transaction struct{}

func (transaction) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	return postgres.TransactionRunWriteBinding(ctx, callback, tenant.WithinCurrentTenant)
}

func (transaction) RunInTenant(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	return postgres.TransactionRunInTenantBinding(ctx, tenantID, callback, tenant.WithinTenant)
}

func (transaction) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

func (transaction) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (transaction) Detach(ctx context.Context) context.Context {
	return postgres.DetachedContext(ctx)
}
