package audit

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Actions recorded in audit.file_events (#2596). Mirrored by the CHECK
// constraint on the table.
const (
	FileEventFolderCreated = "folder_created"
	FileEventFolderUpdated = "folder_updated"
	FileEventFolderDeleted = "folder_deleted"
	FileEventFileUploaded  = "file_uploaded"
	FileEventFileDeleted   = "file_deleted"
)

// FileEvent is one append-only entry of the school file storage trail: who
// created, changed or deleted a folder, who uploaded or deleted a file.
//
// FolderID and FileID carry no foreign key on purpose: the trail must outlive
// the folder cascade. The actor name is snapshotted so the row still reads
// after a rename or account deletion.
type FileEvent struct {
	base.Model `bun:"schema:audit,table:file_events"`
	base.TenantModel
	FolderID       *int64 `bun:"folder_id" json:"folder_id,omitempty"`
	FileID         *int64 `bun:"file_id" json:"file_id,omitempty"`
	Action         string `bun:"action,notnull" json:"action"`
	ActorAccountID *int64 `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	ActorName      string `bun:"actor_name,notnull" json:"actor_name"`
	Detail         string `bun:"detail,notnull" json:"detail"`
}

// Validate ensures the event row is storable.
func (e *FileEvent) Validate() error {
	switch e.Action {
	case FileEventFolderCreated, FileEventFolderUpdated, FileEventFolderDeleted,
		FileEventFileUploaded, FileEventFileDeleted:
	default:
		return errors.New("unknown file event action")
	}
	if strings.TrimSpace(e.ActorName) == "" {
		return errors.New("actor_name is required")
	}
	if strings.TrimSpace(e.Detail) == "" {
		return errors.New("detail is required")
	}
	return nil
}

// FileEventRepository appends to and reads the file storage trail.
type FileEventRepository interface {
	Create(ctx context.Context, event *FileEvent) error
	// ListRecent returns the newest events of the tenant, capped at limit.
	ListRecent(ctx context.Context, limit int) ([]*FileEvent, error)
}
