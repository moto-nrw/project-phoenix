package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	documentCompose "github.com/moto-nrw/project-phoenix/modules/documentrendering/compose"
	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// UploadsBackend is the object store the File Storage module writes to; the
// root resolves it from the uploads directory.
type UploadsBackend = filestorageCompose.ObjectBackend

// FileStoreTestModule is the narrow File Storage composition the file and
// attachment route tests drive: the public capability plus the settings
// service that switches staff uploads.
type FileStoreTestModule struct {
	FileStore *filestorageModule.Module
	Settings  config.SettingsService
}

// NewFileStoreTestModule composes the File Storage module over the test
// database and the given object store. These are the staff file and
// attachment routes; parent attachment access is composed and exercised by
// the parent-portal suite.
func NewFileStoreTestModule(db *bun.DB, unit tenant.UnitOfWork, objects UploadsBackend) (FileStoreTestModule, error) {
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
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{
		DB:      db,
		Observe: func(identityaccessCompose.Observation) {},
	})
	if err != nil {
		return FileStoreTestModule{}, err
	}
	persons, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	announcements := communicationCompose.NewParentAnnouncements(communicationCompose.ParentAnnouncementConfig{
		Repo: repositories.NewParentAnnouncementRepository(db, enrollmentAudience.New()), Settings: settings.Settings, Logger: slog.Default(),
	})
	files, err := filestorageCompose.New(filestorageCompose.Dependencies{
		DB:            db,
		Objects:       objects,
		Identity:      identity,
		People:        persons,
		Settings:      fileStorageSettings{service: settings.Settings},
		Events:        fileStorageEvents{repo: repositories.NewFileEventTestRepository(db, command)},
		FileCleanups:  documentCompose.NewFileCleanupStore(db),
		HasPermission: securityruntime.HasPermission,
		Announcements: announcements,
		Observe:       func(filestorageCompose.Observation) {},
		Logger:        slog.Default(),
	})
	if err != nil {
		return FileStoreTestModule{}, err
	}
	announcements.SetAttachmentPurger(files)
	return FileStoreTestModule{FileStore: files, Settings: settings.Settings}, nil
}
