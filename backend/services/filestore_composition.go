package services

import (
	"context"
	"fmt"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/services/config"
)

type fileStorageSettings struct{ service config.SettingsService }

func (s fileStorageSettings) StaffUploadEnabled(ctx context.Context) (bool, error) {
	enabled, err := s.service.ResolveBool(ctx, configModels.KeyFilesStaffUploadEnabled)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModels.KeyFilesStaffUploadEnabled, err)
	}
	return enabled, nil
}

func (s fileStorageSettings) MaxStorageBytes(ctx context.Context) (int64, error) {
	mb, err := s.service.ResolveInt(ctx, configModels.KeyFilesMaxStorageMB)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModels.KeyFilesMaxStorageMB, err)
	}
	return int64(mb) * 1024 * 1024, nil
}

type fileStorageEvents struct {
	repo auditModels.FileEventRepository
}

func (e fileStorageEvents) Record(ctx context.Context, event filestorageCompose.Event) error {
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
	return e.repo.Create(ctx, row)
}
