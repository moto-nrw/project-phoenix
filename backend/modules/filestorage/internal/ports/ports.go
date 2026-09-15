// Package ports declares the consumer-owned seams the File Storage
// application needs: its own tables, the object store, and the foreign facts
// it must ask other owners for. Composition binds them.
package ports

import (
	"context"
	"io"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
)

// FolderStore persists documents.folders and its share lists
// (documents.folder_roles, documents.folder_accounts).
type FolderStore interface {
	Create(ctx context.Context, folder domain.Folder) (domain.Folder, domain.OperationStats, error)
	// Update rewrites name and visibility; a missing folder is domain.ErrNotFound.
	Update(ctx context.Context, folder domain.Folder) (domain.Folder, domain.OperationStats, error)
	// Delete removes the folder row; its files and share lists cascade. The
	// caller queues cleanup intents for the bytes BEFORE calling this.
	Delete(ctx context.Context, folderID int64) (domain.OperationStats, error)
	FindByID(ctx context.Context, folderID int64) (domain.Folder, bool, domain.OperationStats, error)
	// ListVisible returns the folders the viewer may see, by name, with the
	// number of live files in each.
	ListVisible(ctx context.Context, viewer domain.Viewer) ([]domain.FolderListItem, domain.OperationStats, error)
	// IsVisible answers whether the viewer may see one folder.
	IsVisible(ctx context.Context, folderID int64, viewer domain.Viewer) (bool, domain.OperationStats, error)
	// ReplaceAudience rewrites both share lists of a folder.
	ReplaceAudience(ctx context.Context, folderID int64, audience domain.Audience) (domain.OperationStats, error)
	// GetAudience loads the share lists of the given folders.
	GetAudience(ctx context.Context, folderIDs []int64) (map[int64]domain.Audience, domain.OperationStats, error)
}

// DocumentStore persists the metadata rows of one document table:
// documents.files (owner = folder) or documents.announcement_attachments
// (owner = announcement).
type DocumentStore interface {
	Create(ctx context.Context, document domain.NewDocument) (domain.Document, domain.OperationStats, error)
	// FindForOwner loads one document and enforces that it belongs to the
	// given owner. Soft-deleted rows are found only with includeDeleted.
	FindForOwner(ctx context.Context, ownerID, documentID int64, includeDeleted bool) (domain.Document, bool, domain.OperationStats, error)
	// ListByOwner returns the owner's live documents, newest first.
	ListByOwner(ctx context.Context, ownerID int64) ([]domain.Document, domain.OperationStats, error)
	// ListPendingCleanupByOwner returns the owner's documents whose bytes are
	// still stored, soft-deleted or not: the list to queue intents for before
	// the owner is deleted.
	ListPendingCleanupByOwner(ctx context.Context, ownerID int64) ([]domain.Document, domain.OperationStats, error)
	// ListDeletedPendingCleanup returns soft-deleted documents whose bytes
	// still need removal, capped at domain.CleanupBatchSize.
	ListDeletedPendingCleanup(ctx context.Context) ([]domain.Document, domain.OperationStats, error)
	// ListDeletedPendingCleanupByOwner is the request-path retry list, capped
	// at domain.RequestCleanupRetryLimit.
	ListDeletedPendingCleanupByOwner(ctx context.Context, ownerID int64) ([]domain.Document, domain.OperationStats, error)
	// SoftDelete stamps deleted_at/deleted_by on a live row and returns it; a
	// row that is missing or already deleted is domain.ErrNotFound.
	SoftDelete(ctx context.Context, documentID, deletedBy int64) (domain.Document, domain.OperationStats, error)
	// MarkBytesDeleted records that the stored object is gone.
	MarkBytesDeleted(ctx context.Context, documentID int64) (domain.OperationStats, error)
	CountByOwner(ctx context.Context, ownerID int64) (int, domain.OperationStats, error)
	// TotalStoredBytes sums every document whose bytes still occupy the
	// backend, soft-deleted or not.
	TotalStoredBytes(ctx context.Context) (int64, domain.OperationStats, error)
}

// CleanupStore persists the durable cleanup intents of one document table.
type CleanupStore interface {
	// Queue records the intent to remove one object. A settled intent for the
	// same stored name is revived rather than left alone.
	Queue(ctx context.Context, ownerID int64, storedName string, retryAfter time.Time) (domain.OperationStats, error)
	// ListQueued returns the eligible intents of the tenant and locks them for
	// the caller's transaction (FOR UPDATE SKIP LOCKED). The caller removes the
	// objects inside the SAME transaction.
	ListQueued(ctx context.Context) ([]domain.CleanupIntent, domain.OperationStats, error)
	MarkComplete(ctx context.Context, intentID int64) (domain.OperationStats, error)
	MarkCompleteByFilename(ctx context.Context, storedName string) (domain.OperationStats, error)
	// ActivateByFilename makes an intent eligible for retry right away.
	ActivateByFilename(ctx context.Context, storedName string) (domain.OperationStats, error)
}

// Object is an open stored object; http.ServeContent needs exactly this shape.
type Object interface {
	io.ReadSeekCloser
	ModTime() time.Time
}

// Objects moves bytes for one storage kind (files or attachments). Keys are
// {kind}/{tenant}/{stored name}; the adapter owns the layout.
type Objects interface {
	Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error)
	// Open returns domain.ErrObjectNotFound when the bytes are gone.
	Open(ctx context.Context, tenantID int64, storedName string) (Object, error)
	// Remove deletes the object; a missing object counts as removed.
	Remove(ctx context.Context, tenantID int64, storedName string) error
}

// Identity supplies the account and role facts of the school in context.
type Identity interface {
	// HasActiveMembership reports whether the account still holds an active
	// mapping to the school.
	HasActiveMembership(ctx context.Context, accountID int64) (bool, error)
	// ListAccountRoleIDs returns the roles the account holds at this school.
	ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error)
	// ListShareableRoles returns the roles a folder can be shared with, by
	// name: the school's own roles plus the system roles, without the
	// guardian tier.
	ListShareableRoles(ctx context.Context) ([]domain.AudienceRole, error)
	// ListActiveAccountIDs returns every account with an active mapping.
	ListActiveAccountIDs(ctx context.Context) ([]int64, error)
}

// PersonName is the display name behind an account.
type PersonName struct {
	FirstName string
	LastName  string
}

// People resolves the persons behind accounts; accounts without a person of
// the school are absent from the result.
type People interface {
	PersonNames(ctx context.Context, accountIDs []int64) (map[int64]PersonName, error)
}

// Settings resolves the two tenant settings of the file storage.
type Settings interface {
	StaffUploadEnabled(ctx context.Context) (bool, error)
	// MaxStorageBytes returns 0 when the school has no quota.
	MaxStorageBytes(ctx context.Context) (int64, error)
}

// Event is one append-only entry of the file storage trail.
type Event struct {
	FolderID       *int64
	AnnouncementID *int64
	FileID         *int64
	Action         string
	Actor          domain.Actor
	Detail         string
}

// Event actions, mirrored by the CHECK constraint on audit.file_events.
const (
	EventFolderCreated      = "folder_created"
	EventFolderUpdated      = "folder_updated"
	EventFolderDeleted      = "folder_deleted"
	EventFileUploaded       = "file_uploaded"
	EventFileDeleted        = "file_deleted"
	EventAttachmentUploaded = "announcement_attachment_uploaded"
	EventAttachmentDeleted  = "announcement_attachment_deleted"
)

// Events appends to the audit trail inside the caller's transaction.
type Events interface {
	Record(ctx context.Context, event Event) error
}

// Announcements is the staff-side question the file storage cannot answer
// itself: does the announcement exist in the current tenant, and may its
// content still be changed. Implemented by the Communication owner.
type Announcements interface {
	AnnouncementExists(ctx context.Context, announcementID int64) (bool, error)
	AnnouncementEditable(ctx context.Context, announcementID int64) (bool, error)
	// LockAnnouncementForAttachmentChange answers (exists, editable) while
	// holding the announcement's row lock until the caller's transaction ends.
	LockAnnouncementForAttachmentChange(ctx context.Context, announcementID int64) (bool, bool, error)
	// ResetAnnouncementEngagement drops the read and acknowledgement rows of a
	// draft after its attachments changed.
	ResetAnnouncementEngagement(ctx context.Context, announcementID int64) error
}

// GuardianAudience is the parent-side question: is this guardian account in
// the audience of this announcement right now, and which school does the
// announcement belong to. Tenant id 0 with no error means "not for you".
type GuardianAudience interface {
	GuardianAnnouncementTenant(ctx context.Context, accountID, announcementID int64) (int64, error)
}

// Transaction is the tenant-runtime seam.
type Transaction interface {
	// TenantID returns the tenant in context, 0 when absent.
	TenantID(ctx context.Context) int64
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(ctx context.Context, fn func(context.Context) error) error
	// RunInTenant opens a transaction for the named tenant; the parents
	// portal is cross-tenant, so the announcement decides the school.
	RunInTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
	// AcquireLock takes a transaction-scoped advisory lock.
	AcquireLock(ctx context.Context, key string) error
	// AfterCommit runs fn once the surrounding transaction committed, or at
	// once when there is none.
	AfterCommit(ctx context.Context, fn func())
	// Detach returns a context for post-commit work: no cancellation, no
	// transaction, no after-commit hooks, same tenant.
	Detach(ctx context.Context) context.Context
}

// Observation records one operation for the owner's metrics.
type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

// Observer receives observations.
type Observer func(Observation)

// StoredNames generates storage names for validated content types. The
// display name never reaches the backend: a UUID cannot collide, cannot
// carry a traversal payload, and leaks nothing about the content.
type StoredNames func(extension string) (string, error)

// Logger is the structured logger the application reports non-fatal cleanup
// outcomes through.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}
