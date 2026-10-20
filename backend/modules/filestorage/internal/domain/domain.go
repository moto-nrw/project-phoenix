// Package domain holds the values and rules of the school file storage
// (#2596, migrated under #2707): folders with a visibility rule, the files
// inside them, the attachments of Elternmitteilungen (#2890), and the cleanup
// intents that make an interrupted upload recoverable.
//
// The shape is deliberately flat. There is one folder level and no per-file
// rights: a file is visible exactly when its folder is.
package domain

import (
	"errors"
	"io"
	"strings"
	"time"
)

// Folder visibility values, mirrored by the CHECK constraint on
// documents.folders.visibility.
const (
	// VisibilityAllStaff opens the folder to every account with an active
	// mapping to the school.
	VisibilityAllStaff = "all_staff"
	// VisibilityAdmins restricts the folder to admins (admin:* holders) and
	// accounts holding files:manage.
	VisibilityAdmins = "admins"
	// VisibilitySelected shares the folder with the roles and accounts listed
	// in documents.folder_roles / documents.folder_accounts.
	VisibilitySelected = "selected"
)

// IsValidVisibility reports whether v is a known visibility value.
func IsValidVisibility(v string) bool {
	switch v {
	case VisibilityAllStaff, VisibilityAdmins, VisibilitySelected:
		return true
	default:
		return false
	}
}

const (
	// FileCategory is the single category every stored file carries.
	FileCategory = "file"
	// AttachmentCategory is the single category every attachment carries.
	AttachmentCategory = "announcement_attachment"
	// MaxFolderNameLength mirrors the CHECK constraint on documents.folders.name.
	MaxFolderNameLength = 120
	// MaxAnnouncementAttachments begrenzt die Anhänge je Mitteilung. Die Grenze
	// ist eine Produktentscheidung, keine technische: eine Elternmitteilung mit
	// mehr als einer Handvoll Dateien ist keine Mitteilung mehr.
	MaxAnnouncementAttachments = 5
	// CleanupBatchSize caps one storage-cleanup sweep pass. The sweep runs
	// inside a single tenant transaction, so an uncapped pass after a large
	// deletion would hold a pool connection through thousands of
	// unlink-and-mark pairs. Bounded passes commit their progress and resume
	// on the next tick.
	CleanupBatchSize = 200
	// RequestCleanupRetryLimit caps how many stale objects a single page view
	// will try to reclaim. A request should mop up the odd straggler, never
	// work through a backlog; that belongs to the scheduler.
	RequestCleanupRetryLimit = 10
	// UploadDeadline bounds every step an upload may still perform once its
	// cleanup intent exists (object write + metadata transaction).
	UploadDeadline = 2 * time.Minute
	// CleanupDelay keeps a queued intent ineligible until well past
	// UploadDeadline. It MUST stay larger than UploadDeadline.
	CleanupDelay = 5 * time.Minute
)

// Actor identifies the acting account for authority checks and audit rows.
type Actor struct {
	AccountID int64
	Name      string
	// Manager is true for admins and files:manage holders: they see every
	// folder regardless of visibility and manage folders and every file.
	Manager bool
}

// DisplayName is the actor's name as the file event trail records it; an
// actor without a name is "Unbekannt".
func (a Actor) DisplayName() string {
	if name := strings.TrimSpace(a.Name); name != "" {
		return name
	}
	return "Unbekannt"
}

// Folder is one folder of the school storage.
type Folder struct {
	ID         int64
	TenantID   int64
	Name       string
	Visibility string
	CreatedBy  int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// FolderListItem is a folder together with its live file count.
type FolderListItem struct {
	Folder
	FileCount int64
}

// Audience is the explicit share list of a folder with VisibilitySelected.
type Audience struct {
	RoleIDs    []int64
	AccountIDs []int64
}

// Viewer describes the caller against whom folder visibility is resolved.
type Viewer struct {
	AccountID int64
	Manager   bool
	// RoleIDs are the roles the account holds AT THIS SCHOOL, so a role held
	// at another school never opens a folder here.
	RoleIDs []int64
}

// Document is the metadata of one stored object: a file in a folder or an
// attachment of an announcement. OwnerID is the folder or the announcement.
//
// DeletedAt is a soft delete on purpose: deleting removes the bytes right
// away while the row survives as the record that the document once existed.
// FileDeletedAt records that the bytes are actually gone, so a failed unlink
// can be retried without guessing.
type Document struct {
	ID              int64
	TenantID        int64
	OwnerID         int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
	UploadedBy      int64
	CreatedAt       time.Time
	DeletedAt       *time.Time
	DeletedBy       *int64
	FileDeletedAt   *time.Time
}

// BytesPresent reports whether the stored object still occupies the backend.
func (d Document) BytesPresent() bool { return d.FileDeletedAt == nil }

// NewDocument carries the metadata of an already-stored upload.
type NewDocument struct {
	OwnerID         int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
	UploadedBy      int64
}

// Validate ensures the row is storable.
func (d *NewDocument) Validate() error {
	d.FilenameDisplay = strings.TrimSpace(d.FilenameDisplay)
	switch {
	case d.OwnerID <= 0:
		return errors.New("owner is required")
	case strings.TrimSpace(d.Category) == "":
		return errors.New("category is required")
	case d.FilenameDisplay == "":
		return errors.New("filename_display is required")
	case strings.TrimSpace(d.FilenameStored) == "":
		return errors.New("filename_stored is required")
	case d.SizeBytes < 0:
		return errors.New("size_bytes must not be negative")
	case strings.TrimSpace(d.ContentType) == "":
		return errors.New("content_type is required")
	case d.UploadedBy <= 0:
		return errors.New("uploaded_by is required")
	}
	return nil
}

// CleanupIntent is a durable intent to remove stored bytes. It is written
// BEFORE the object is saved, so a process that dies between the write and
// the metadata commit still leaves a record the sweep can act on. It carries
// no foreign key to its owner: the owner may be deleted meanwhile and the
// orphaned bytes still have to go.
type CleanupIntent struct {
	ID             int64
	TenantID       int64
	OwnerID        int64
	FilenameStored string
	RetryAfter     time.Time
	CleanedAt      *time.Time
}

// DedupeIDs drops non-positive and repeated ids, keeping first-seen order.
func DedupeIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// FolderView is a folder as the list endpoint returns it: the row, its live
// file count and (for managers) its share list.
type FolderView struct {
	FolderListItem
	Audience Audience
}

// Overview is the response of the folder list.
type Overview struct {
	Folders []FolderView
	// CanManage is true for admins and files:manage holders.
	CanManage bool
	// CanUpload is true when the actor may add files to the folders they see.
	CanUpload bool
	// StaffUploadEnabled mirrors files.staff_upload_enabled so the UI can
	// explain who uploads when the actor cannot.
	StaffUploadEnabled bool
	UsedBytes          int64
	MaxBytes           int64
}

// FileView is a listed file together with the caller's delete right.
type FileView struct {
	Document
	CanDelete bool
}

// Upload is the validated multipart input of one upload. Extension is the
// canonical stored-file extension of the validated content type.
type Upload struct {
	FilenameDisplay string
	ContentType     string
	Extension       string
	Content         io.Reader
}

// OperationStats counts the persistence work of one operation.
type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

// Add accumulates another statement's stats.
func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
