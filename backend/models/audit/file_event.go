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

	// Anhänge an Elternmitteilungen (#2890). Ein Anhang ist eine Datei und
	// gehört in dieselbe Spur wie die Dateiablage — nicht in eine zweite
	// Audit-Tabelle. Diese Zeilen tragen announcement_id statt folder_id.
	FileEventAnnouncementAttachmentUploaded = "announcement_attachment_uploaded"
	FileEventAnnouncementAttachmentDeleted  = "announcement_attachment_deleted"
)

// FileEvent is one append-only entry of the school file storage trail: who
// created, changed or deleted a folder, who uploaded or deleted a file, and
// who attached a file to an Elternmitteilung (#2890).
//
// FolderID, AnnouncementID and FileID carry no foreign key on purpose: the
// trail must outlive the folder and announcement cascades. The actor name is
// snapshotted so the row still reads after a rename or account deletion.
// Exactly one of FolderID / AnnouncementID is set, depending on the action.
type FileEvent struct {
	base.Model `bun:"schema:audit,table:file_events"`
	base.TenantModel
	FolderID       *int64 `bun:"folder_id" json:"folder_id,omitempty"`
	AnnouncementID *int64 `bun:"announcement_id" json:"announcement_id,omitempty"`
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
		FileEventFileUploaded, FileEventFileDeleted,
		FileEventAnnouncementAttachmentUploaded, FileEventAnnouncementAttachmentDeleted:
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
