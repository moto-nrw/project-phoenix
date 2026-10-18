//go:build testhooks

package api

import (
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	enrollmentAudience "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// UploadsBackend is the object store the File Storage module writes to; the
// root resolves it from the uploads directory.
type UploadsBackend = filestorageCompose.ObjectBackend

// FileStoreTestModule is the narrow File Storage composition the file and
// attachment route tests drive: the public capability plus the settings
// service that switches staff uploads.
type FileStoreTestModule struct {
	FileStore *filestorageModule.Module
	Settings  settingsCompose.SettingsService
}

// NewFileStoreTestModule composes the File Storage module over the test
// database and the given object store. These are the staff file and
// attachment routes; parent attachment access is composed and exercised by
// the parent-portal suite.
func NewFileStoreTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, objects UploadsBackend) (FileStoreTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	command, err := auditCompose.NewCommand(
		NewTestAuditStore(db),
		func(auditCompose.AppendObservation) {},
	)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(identityaccessCompose.Observation) {},
	})
	if err != nil {
		return FileStoreTestModule{}, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return FileStoreTestModule{}, err
	}
	purger, err := filestorageCompose.NewAttachmentPurger(filestorageCompose.AttachmentPurgerDependencies{
		DB: db.Pool(), Objects: objects, Observe: func(filestorageCompose.Observation) {},
	})
	if err != nil {
		return FileStoreTestModule{}, err
	}
	announcements, err := communicationCompose.NewParentAnnouncementsWithAttachments(communicationCompose.ParentAnnouncementConfig{
		Repo: parentStore.NewParentAnnouncementRepository(db.Pool(), ParentAnnouncementApplicants(enrollmentAudience.New()), guardianLinkView.NewReader()), Settings: settingsValues.NewParentAnnouncementValues(settings.Settings), Logger: slog.Default(),
		Fingerprints: SecurityFingerprints{},
	}, purger, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	if err != nil {
		return FileStoreTestModule{}, err
	}
	files, err := filestorageCompose.New(filestorageCompose.Dependencies{
		DB:            db.Pool(),
		Objects:       objects,
		Identity:      identity,
		People:        persons,
		Settings:      FileStorageSettings{Service: settings.Settings},
		Events:        FileStorageEvents{Repo: newFileEventTestRepository(db, command)},
		HasPermission: securityruntime.HasPermission,
		Announcements: announcements,
		Observe:       func(filestorageCompose.Observation) {},
		Logger:        slog.Default(),
	})
	if err != nil {
		return FileStoreTestModule{}, err
	}
	return FileStoreTestModule{FileStore: files, Settings: settings.Settings}, nil
}
