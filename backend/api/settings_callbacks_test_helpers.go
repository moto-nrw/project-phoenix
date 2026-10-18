//go:build testhooks

package api

import (
	"context"
	"log/slog"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SettingsCallbacksTestModule is the Settings Platform's tenant settings
// with the side effects of a settings change registered.
type SettingsCallbacksTestModule struct {
	Settings       settingsCompose.SettingsService
	TenantSettings configModel.TenantOperationsService
}

// NewSettingsCallbacksTestModule composes the tenant settings over db and
// returns them with the realtime hub a settings change broadcasts on.
func NewSettingsCallbacksTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, unlinker peoplePorts.PhotoUnlinker) (SettingsCallbacksTestModule, *realtime.Hub, error) {
	effects, _, err := newOperatorSettingSideEffects(db, unit)
	if err != nil {
		return SettingsCallbacksTestModule{}, nil, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return SettingsCallbacksTestModule{}, nil, err
	}
	hub := deliveryCompose.NewRealtimeHub(slog.Default())
	photoDeps := StudentPhotoRuntimeDependencies{
		Settings: settings.Settings, Broadcaster: hub, Unlinker: unlinker, Logger: slog.Default(),
		ActingAccount: actingAccountID,
	}
	directory, err := peopleCompose.New(peopleCompose.Dependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               db.Pool(), Observe: func(peopleCompose.Observation) {},
		StudentPhotos: NewStudentPhotoRuntime(photoDeps),
	})
	if err != nil {
		return SettingsCallbacksTestModule{}, nil, err
	}
	photos := NewStudentPhotos(directory, photoDeps)
	registry, err := settingsCompose.NewSideEffects(func(string) {}, append(effects,
		settingsCompose.SettingSideEffect{Key: configModel.KeyStudentPhotosEnabled, Handle: photos.HandleFeatureToggle})...)
	if err != nil {
		return SettingsCallbacksTestModule{}, nil, err
	}
	operations := settingsCompose.NewTenantOperations(settings.Settings, settings.payroll, settings.runtime,
		registry.Dispatch,
		func(_ context.Context, tenantID int64, key string) {
			_ = hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key}))
		})
	return SettingsCallbacksTestModule{Settings: settings.Settings, TenantSettings: operations}, hub, nil
}

// NewStudentPhotos binds the owner capability and the runtime it needs, and
// returns the retained contract the handlers call. The directory is composed
// over NewStudentPhotoRuntime with the same dependencies.
func NewStudentPhotos(directory StudentPhotoCapability, deps StudentPhotoRuntimeDependencies) peoplePorts.StudentPhotoService {
	return studentPhotos{directory: directory, runtime: newStudentPhotoRuntime(deps)}
}

// NewStudentPhotoRuntime is the photo runtime a People Directory is composed
// with (peopleCompose.Dependencies.StudentPhotos).
func NewStudentPhotoRuntime(deps StudentPhotoRuntimeDependencies) peopleCompose.StudentPhotoRuntime {
	return newStudentPhotoRuntime(deps)
}

func newStudentPhotoRuntime(deps StudentPhotoRuntimeDependencies) studentPhotoRuntime {
	return studentPhotoRuntime{
		settings: deps.Settings, broadcaster: deps.Broadcaster,
		unlinker: deps.Unlinker, consents: deps.Consents, logger: deps.Logger,
		actor: deps.ActingAccount,
	}
}
