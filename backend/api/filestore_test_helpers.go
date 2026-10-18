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

// NewFileStoreTestModule composes the narrow File Storage composition the
// file and attachment route tests drive over the test database and the given
// object store: the public capability plus the settings service that switches
// staff uploads. These are the staff file and attachment routes; parent
// attachment access is composed and exercised by the parent-portal suite.
func NewFileStoreTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, objects UploadsBackend) (*filestorageModule.Module, settingsCompose.SettingsService, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return nil, nil, err
	}
	command, err := auditCompose.NewCommand(
		NewTestAuditStore(db),
		func(auditCompose.AppendObservation) {},
	)
	if err != nil {
		return nil, nil, err
	}
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(identityaccessCompose.Observation) {},
	})
	if err != nil {
		return nil, nil, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, nil, err
	}
	purger, err := filestorageCompose.NewAttachmentPurger(filestorageCompose.AttachmentPurgerDependencies{
		DB: db.Pool(), Objects: objects, Observe: func(filestorageCompose.Observation) {},
	})
	if err != nil {
		return nil, nil, err
	}
	announcements, err := communicationCompose.NewParentAnnouncementsWithAttachments(communicationCompose.ParentAnnouncementConfig{
		Repo: parentStore.NewParentAnnouncementRepository(db.Pool(), newParentAnnouncementAudience(db, ParentAnnouncementApplicants(enrollmentAudience.New()))), Settings: settingsValues.NewParentAnnouncementValues(settings.Settings), Logger: slog.Default(),
		Fingerprints: SecurityFingerprints{},
	}, purger, CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}
	return files, settings.Settings, nil
}
