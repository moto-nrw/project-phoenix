package owners

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// FileStorageSettings resolves the tenant's file storage settings.
type FileStorageSettings struct {
	Service settingsCompose.SettingsService
}

func (s FileStorageSettings) StaffUploadEnabled(ctx context.Context) (bool, error) {
	enabled, err := s.Service.ResolveBool(ctx, configModels.KeyFilesStaffUploadEnabled)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModels.KeyFilesStaffUploadEnabled, err)
	}
	return enabled, nil
}

func (s FileStorageSettings) MaxStorageBytes(ctx context.Context) (int64, error) {
	mb, err := s.Service.ResolveInt(ctx, configModels.KeyFilesMaxStorageMB)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModels.KeyFilesMaxStorageMB, err)
	}
	return int64(mb) * 1024 * 1024, nil
}

// FileStorageEvents records the File Storage events in the Audit file event
// trail.
type FileStorageEvents struct {
	Repo FileEventAppender
}

func (e FileStorageEvents) Record(ctx context.Context, event filestorageCompose.Event) error {
	row := &auditModels.FileEvent{
		FolderID:       event.FolderID,
		AnnouncementID: event.AnnouncementID,
		FileID:         event.FileID,
		Action:         event.Action,
		ActorName:      event.Actor.DisplayName(),
		Detail:         event.Detail,
	}
	if event.Actor.AccountID > 0 {
		id := event.Actor.AccountID
		row.ActorAccountID = &id
	}
	return e.Repo.Create(ctx, row)
}
