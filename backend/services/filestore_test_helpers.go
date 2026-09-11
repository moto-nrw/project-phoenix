package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/filestore"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type FileStoreTestModule struct {
	FileStore filestore.Service
	Settings  config.SettingsService
}

func NewFileStoreTestModule(db *bun.DB, unit tenant.UnitOfWork) (FileStoreTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	command, err := auditSvc.NewCommand(
		repositories.NewTestAuditStore(db),
		func(auditSvc.AppendObservation) {},
	)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	repos, err := repositories.NewFileStoreTestRepositories(db, command)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	files := filestore.NewService(db, repos.Folders, repos.Files, repos.Attachments, repos.Events, settings.Settings, slog.Default())
	announcements := communicationCompose.NewParentAnnouncements(communicationCompose.ParentAnnouncementConfig{
		Repo: repos.Announcements, Settings: settings.Settings, Logger: slog.Default(),
	})
	// These are the staff file and attachment routes. Parent attachment access
	// is composed and exercised by the parent-portal suite.
	files.(filestore.AnnouncementPortSetter).SetAnnouncementPorts(announcements, nil)
	announcements.SetAttachmentPurger(files)
	return FileStoreTestModule{FileStore: files, Settings: settings.Settings}, nil
}
