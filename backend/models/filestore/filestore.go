// Package filestore holds the school-wide file storage (#2596): folders with
// a visibility rule, the files inside them, and the cleanup intents that make
// an interrupted upload recoverable.
//
// The shape is deliberately flat. There is one folder level and no per-file
// rights: a file is visible exactly when its folder is. Everything a child or
// staff document already knows about safe uploads (models/documents) is
// reused rather than restated.
package filestore

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/documents"
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

// Visibilities lists every valid visibility value in display order.
var Visibilities = []string{VisibilityAllStaff, VisibilityAdmins, VisibilitySelected}

// IsValidVisibility reports whether v is a known visibility value.
func IsValidVisibility(v string) bool {
	switch v {
	case VisibilityAllStaff, VisibilityAdmins, VisibilitySelected:
		return true
	default:
		return false
	}
}

// FileCategory is the single category every stored file carries. The shared
// document repository filters by category; a storage without categories
// simply has one.
const FileCategory = "file"

// MaxFolderNameLength mirrors the CHECK constraint on documents.folders.name.
const MaxFolderNameLength = 120

// Folder is one folder of the school storage.
type Folder struct {
	base.Model `bun:"schema:documents,table:folders"`
	base.TenantModel
	Name       string `bun:"name,notnull" json:"name"`
	Visibility string `bun:"visibility,notnull" json:"visibility"`
	CreatedBy  int64  `bun:"created_by,notnull" json:"created_by"`
}

// Validate ensures the folder row is storable.
func (f *Folder) Validate() error {
	f.Name = strings.TrimSpace(f.Name)
	if f.Name == "" {
		return errors.New("name is required")
	}
	if len([]rune(f.Name)) > MaxFolderNameLength {
		return errors.New("name is too long")
	}
	if !IsValidVisibility(f.Visibility) {
		return errors.New("unknown visibility")
	}
	if f.CreatedBy <= 0 {
		return errors.New("created_by is required")
	}
	return nil
}

// FolderRole shares a folder with one role of the school.
type FolderRole struct {
	base.TenantModel `bun:"schema:documents,table:folder_roles"`
	FolderID         int64 `bun:"folder_id,notnull"`
	RoleID           int64 `bun:"role_id,notnull"`
}

// FolderAccount shares a folder with one account of the school.
type FolderAccount struct {
	base.TenantModel `bun:"schema:documents,table:folder_accounts"`
	FolderID         int64 `bun:"folder_id,notnull"`
	AccountID        int64 `bun:"account_id,notnull"`
}

// Audience is the explicit share list of a folder with VisibilitySelected.
type Audience struct {
	RoleIDs    []int64
	AccountIDs []int64
}

// File is one stored file. Only metadata — the bytes live in the storage
// backend under the stored (UUID) name and are served exclusively through the
// visibility-checked download handler.
type File struct {
	documents.File `bun:"schema:documents,table:files"`
	FolderID       int64 `bun:"folder_id,notnull" json:"folder_id"`
}

// GetOwnerID satisfies documents.Entity: the owner of a file is its folder.
func (f *File) GetOwnerID() int64 { return f.FolderID }

// Validate ensures the file row is storable.
func (f *File) Validate() error {
	if f.FolderID <= 0 {
		return errors.New("folder_id is required")
	}
	if f.Category != FileCategory {
		return errors.New("unknown file category")
	}
	return documents.ValidateFile(&f.File)
}

// FileCleanup tracks an upload whose metadata could not be committed, or
// whose folder was deleted before the bytes were removed. No folder foreign
// key on purpose: the folder cascade must not take the intent with it.
type FileCleanup struct {
	documents.FileCleanup `bun:"schema:documents,table:file_cleanup"`
}

// AudienceRole is one role a folder can be shared with.
type AudienceRole struct {
	ID   int64  `bun:"id" json:"id"`
	Name string `bun:"name" json:"name"`
}

// AudienceAccount is one account (a person of the school) a folder can be
// shared with.
type AudienceAccount struct {
	AccountID int64  `bun:"account_id" json:"account_id"`
	FirstName string `bun:"first_name" json:"first_name"`
	LastName  string `bun:"last_name" json:"last_name"`
}

// FolderListItem is a folder together with the derived numbers the list view
// shows.
type FolderListItem struct {
	Folder
	FileCount int64 `bun:"file_count" json:"file_count"`
}

// Viewer describes the caller against whom folder visibility is resolved.
type Viewer struct {
	AccountID int64
	// Manager is true for admins and files:manage holders: they see every
	// folder regardless of visibility.
	Manager bool
}

// FolderRepository persists folders and their share lists.
type FolderRepository interface {
	Create(ctx context.Context, folder *Folder) error
	Update(ctx context.Context, folder *Folder) error
	// Delete removes the folder row. Its files cascade away; the caller must
	// queue cleanup intents for their bytes BEFORE calling this.
	Delete(ctx context.Context, folderID int64) error
	FindByID(ctx context.Context, folderID int64) (*Folder, error)
	// ListVisible returns the folders the viewer may see, by name, with the
	// number of live files in each.
	ListVisible(ctx context.Context, viewer Viewer) ([]*FolderListItem, error)
	// IsVisible answers whether the viewer may see one folder.
	IsVisible(ctx context.Context, folderID int64, viewer Viewer) (bool, error)
	// ReplaceAudience rewrites the share list of a folder.
	ReplaceAudience(ctx context.Context, folderID int64, audience Audience) error
	// GetAudience loads the share lists of the given folders.
	GetAudience(ctx context.Context, folderIDs []int64) (map[int64]Audience, error)
	// ListAudienceRoles returns the roles a folder can be shared with: the
	// school's own roles plus the system roles.
	ListAudienceRoles(ctx context.Context) ([]*AudienceRole, error)
	// ListAudienceAccounts returns the accounts with an active mapping to the
	// school that belong to a person, by name.
	ListAudienceAccounts(ctx context.Context) ([]*AudienceAccount, error)
}

// FileRepository persists file metadata. It is the shared document
// repository (soft delete, cleanup intents) plus the storage-wide quota read.
type FileRepository interface {
	Create(ctx context.Context, file *File) error
	FindForOwner(ctx context.Context, folderID, fileID int64) (*File, error)
	FindForOwnerIncludingDeleted(ctx context.Context, folderID, fileID int64) (*File, error)
	ListByOwnerID(ctx context.Context, folderID int64, categories []string) ([]*File, error)
	ListPendingFileCleanupByOwnerID(ctx context.Context, folderID int64) ([]*File, error)
	ListDeletedPendingFileCleanups(ctx context.Context) ([]*File, error)
	ListDeletedPendingFileCleanupByOwnerID(ctx context.Context, folderID int64, categories []string) ([]*File, error)
	SoftDelete(ctx context.Context, file *File, deletedBy int64) error
	MarkFileDeleted(ctx context.Context, fileID int64) error
	QueueFileCleanup(ctx context.Context, cleanup *FileCleanup) error
	ListQueuedFileCleanups(ctx context.Context) ([]*FileCleanup, error)
	ListQueuedFileCleanupByOwnerID(ctx context.Context, folderID int64) ([]*FileCleanup, error)
	MarkQueuedFileCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedFileCleanupCompleteByFilename(ctx context.Context, filename string) error
	ActivateQueuedFileCleanupByFilename(ctx context.Context, filename string) error
	// TotalStoredBytes sums the size of every file whose bytes are still on
	// the storage backend, soft-deleted or not — that is what occupies the
	// quota until the sweep has removed it.
	TotalStoredBytes(ctx context.Context) (int64, error)
}
