// Package filestorage is the public File Storage capability (#2707, ADR
// 0010). It owns the managed file lifecycle: the school file storage's
// folders with their visibility rule, the files inside them, the attachments
// of Elternmitteilungen, the storage quota, the audit trail, and the cleanup
// intents that make an interrupted upload recoverable. Identity supplies
// account and role facts; Communication decides who may see an announcement's
// attachments; this owner applies both.
package filestorage

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrNotFound marks a folder or file the caller cannot reach: missing, or
	// invisible to them. The two are indistinguishable so ids cannot be probed.
	ErrNotFound = errors.New("file storage item not found")
	// ErrForbidden marks an action the caller may not perform.
	ErrForbidden = errors.New("file storage action not permitted")
	// ErrInvalid marks a semantically invalid request.
	ErrInvalid = errors.New("invalid file storage request")
	// ErrFolderNameTaken marks a duplicate folder name.
	ErrFolderNameTaken = errors.New("folder name already exists")
	// ErrQuotaExceeded marks an upload the school's storage quota does not admit.
	ErrQuotaExceeded = errors.New("file storage quota exceeded")
	// ErrObjectNotFound marks stored bytes that are gone although the row exists.
	ErrObjectNotFound = errors.New("stored object not found")
	// ErrAttachmentNotFound marks a missing attachment or an announcement the
	// caller may not see. Ein Konto außerhalb des Empfängerkreises bekommt
	// bewusst 404 und nicht 403.
	ErrAttachmentNotFound = errors.New("announcement attachment not found")
	// ErrAttachmentPublished marks an attempt to change the attachments of an
	// already published announcement; the correction path is unpublish, edit,
	// republish.
	ErrAttachmentPublished = errors.New("announcement is published; attachments are fixed")
	// ErrAttachmentLimitReached marks an upload past MaxAnnouncementAttachments.
	ErrAttachmentLimitReached = errors.New("announcement attachment limit reached")
	// ErrTenantRequired marks an operation without a tenant in context.
	ErrTenantRequired = errors.New("tenant is required")
)

// ErrorCode maps a capability error to the stable code recorded in metrics.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrAttachmentNotFound), errors.Is(err, ErrObjectNotFound):
		return "not_found"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrInvalid):
		return "invalid"
	case errors.Is(err, ErrFolderNameTaken), errors.Is(err, ErrQuotaExceeded),
		errors.Is(err, ErrAttachmentPublished), errors.Is(err, ErrAttachmentLimitReached):
		return "conflict"
	case errors.Is(err, ErrTenantRequired):
		return "tenant_required"
	default:
		return "internal"
	}
}

// MaxAnnouncementAttachments caps the attachments of one announcement.
const MaxAnnouncementAttachments = 5

// Actor identifies the acting account. Permissions decide whether the actor
// manages the storage; the owner applies that rule.
type Actor struct {
	AccountID   int64
	Name        string
	Permissions []string
}

// Folder is one folder with what the caller may know about it: the share
// lists are filled for managers only.
type Folder struct {
	ID         int64
	Name       string
	Visibility string
	FileCount  int64
	RoleIDs    []int64
	AccountIDs []int64
	CreatedAt  time.Time
}

// FolderOverview is the folder list plus what the caller may do, so the UI
// never has to guess authority.
type FolderOverview struct {
	Folders            []Folder
	CanManage          bool
	CanUpload          bool
	StaffUploadEnabled bool
	UsedBytes          int64
	MaxBytes           int64
}

// FolderInput carries a folder create or update. nil and empty share lists
// are different requests: nil means "no list sent".
type FolderInput struct {
	Name       string
	Visibility string
	RoleIDs    []int64
	AccountIDs []int64
}

// File is one stored file as the caller may see it.
type File struct {
	ID          int64
	FolderID    int64
	Filename    string
	SizeBytes   int64
	ContentType string
	UploadedAt  time.Time
	UploadedBy  int64
	CanDelete   bool
}

// FileList is the file list of one folder.
type FileList struct {
	Folder Folder
	Files  []File
}

// Upload is one validated upload: the display name the uploader saw, the
// content type the transport verified by magic bytes, its canonical stored
// extension, and the bytes.
type Upload struct {
	Filename    string
	ContentType string
	Extension   string
	Content     io.Reader
}

// Object is an open stored object: random access over a known size, so a
// transport can stream it with range support and close it afterwards.
type Object interface {
	io.ReaderAt
	Size() int64
	Close() error
}

// Content is an open stored object ready to be served. The caller closes it.
type Content struct {
	Object      Object
	ModTime     time.Time
	Filename    string
	ContentType string
}

// Close releases the underlying object.
func (c Content) Close() error {
	if c.Object == nil {
		return nil
	}
	return c.Object.Close()
}

// AudienceRole is one role a folder can be shared with.
type AudienceRole struct {
	ID   int64
	Name string
}

// AudienceAccount is one person of the school a folder can be shared with.
type AudienceAccount struct {
	AccountID int64
	FirstName string
	LastName  string
}

// AudienceOptions are the roles and persons a folder can be shared with.
type AudienceOptions struct {
	Roles    []AudienceRole
	Accounts []AudienceAccount
}

// Attachment is one attachment of an Elternmitteilung.
type Attachment struct {
	ID             int64
	AnnouncementID int64
	Filename       string
	SizeBytes      int64
	ContentType    string
	UploadedAt     time.Time
}

// AttachmentList is the attachment list of one announcement. Editable is
// false once the announcement is published; it stays true when only the
// limit is reached, because removing must remain possible.
type AttachmentList struct {
	Attachments []Attachment
	Editable    bool
}

// FolderQuery reads folders and their audience options.
type FolderQuery interface {
	ListFolders(ctx context.Context, actor Actor) (FolderOverview, error)
	ListAudienceOptions(ctx context.Context, actor Actor) (AudienceOptions, error)
}

// FolderCommand manages folders; every command requires files:manage.
type FolderCommand interface {
	CreateFolder(ctx context.Context, input FolderInput, actor Actor) (Folder, error)
	UpdateFolder(ctx context.Context, folderID int64, input FolderInput, actor Actor) (Folder, error)
	// DeleteFolder queues cleanup for every file still on disk and removes the
	// folder; the file rows cascade. Bytes are removed by the sweep.
	DeleteFolder(ctx context.Context, folderID int64, actor Actor) error
}

// FileQuery reads the files of visible folders.
type FileQuery interface {
	// ListFiles returns the folder and its live files, newest first.
	ListFiles(ctx context.Context, folderID int64, actor Actor) (FileList, error)
	// OpenFile resolves a file the actor may see and opens its bytes.
	OpenFile(ctx context.Context, folderID, fileID int64, actor Actor) (Content, error)
}

// FileCommand stores and removes files.
type FileCommand interface {
	// UploadFile stores one upload with the intent protocol and returns its
	// metadata. Authority: manager, or staff upload enabled for a visible folder.
	UploadFile(ctx context.Context, folderID int64, upload Upload, actor Actor) (File, error)
	// DeleteFile soft-deletes a file with an audit row and removes its bytes
	// after the surrounding transaction commits.
	DeleteFile(ctx context.Context, folderID, fileID int64, actor Actor) error
}

// AttachmentQuery reads the attachments of Elternmitteilungen.
type AttachmentQuery interface {
	// ListAttachments returns the live attachments for a staff caller, oldest
	// first, and whether the announcement may still be changed.
	ListAttachments(ctx context.Context, announcementID int64) (AttachmentList, error)
	OpenAttachment(ctx context.Context, announcementID, attachmentID int64) (Content, error)
}

// TenantAttachmentReads read an announcement's attachments under the named
// school's RLS, for a caller that has already decided the audience; they check
// nothing else. An invalid id or a missing attachment is ErrAttachmentNotFound.
// They are not part of Capability: only the parents portal's attachment entry
// receives them, through compose.TenantAttachmentReads (#4825).
type TenantAttachmentReads interface {
	ListAnnouncementAttachmentsInTenant(ctx context.Context, tenantID, announcementID int64) ([]Attachment, error)
	FindAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (Attachment, error)
	OpenAnnouncementAttachmentInTenant(ctx context.Context, tenantID, announcementID, attachmentID int64) (Content, error)
}

// AttachmentCommand changes the attachments of a draft announcement.
type AttachmentCommand interface {
	UploadAttachment(ctx context.Context, announcementID int64, upload Upload, actor Actor) (Attachment, error)
	DeleteAttachment(ctx context.Context, announcementID, attachmentID int64, actor Actor) error
}

// AttachmentPurger is the promise the Communication owner relies on: intents
// for every attachment before an announcement is deleted, inside the caller's
// transaction, and the live count at publish time.
type AttachmentPurger interface {
	QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error
	CountAttachments(ctx context.Context, announcementID int64) (int, error)
	// AttachmentDigests reads every live attachment through digest, in the
	// caller's transaction. An Erklärung freezes the digests at publication
	// (#3430); the caller supplies the hash, File Storage only reads.
	AttachmentDigests(ctx context.Context, announcementID int64, digest func(io.Reader) (string, int64, error)) ([]AttachmentDigest, error)
}

// AttachmentDigest identifies one attachment by its content. It is an alias
// of an unnamed struct on purpose: Communication declares the identical
// shape, so this module satisfies Communication's port without either module
// importing the other.
type AttachmentDigest = struct {
	AttachmentID int64
	Filename     string
	ContentType  string
	SizeBytes    int64
	SHA256       string
}

// Cleaner is the scheduler's entry point: it removes objects whose metadata
// never committed or was cascaded away, in the caller's tenant transaction.
type Cleaner interface {
	CleanupOrphanedFiles(ctx context.Context) (int, error)
}

// Query is every read of the capability.
type Query interface {
	FolderQuery
	FileQuery
	AttachmentQuery
}

// Command is every write of the capability.
type Command interface {
	FolderCommand
	FileCommand
	AttachmentCommand
	AttachmentPurger
	Cleaner
}

// Capability is the whole public surface.
type Capability interface {
	Query
	Command
}

// InlineViewable reports whether a browser can render the content type
// itself. Office containers are deliberately absent: the browser would offer
// a download anyway, and an inline disposition for them only invites a
// plugin to run.
func InlineViewable(contentType string) bool {
	switch contentType {
	case "application/pdf", "image/png", "image/jpeg", "image/jpg":
		return true
	default:
		return false
	}
}
