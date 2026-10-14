package services

import (
	"context"
	"fmt"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
)

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

type FileStorageEvents struct {
	Repo auditModels.FileEventRepository
}

func (e FileStorageEvents) Record(ctx context.Context, event filestorageCompose.Event) error {
	name := strings.TrimSpace(event.Actor.Name)
	if name == "" {
		name = "Unbekannt"
	}
	row := &auditModels.FileEvent{
		FolderID:       event.FolderID,
		AnnouncementID: event.AnnouncementID,
		FileID:         event.FileID,
		Action:         event.Action,
		ActorName:      name,
		Detail:         event.Detail,
	}
	if event.Actor.AccountID > 0 {
		id := event.Actor.AccountID
		row.ActorAccountID = &id
	}
	return e.Repo.Create(ctx, row)
}
