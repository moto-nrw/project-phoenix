//go:build testhooks

package api

import (
	"context"
	"log/slog"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SettingsCallbacksTestModule is the Settings Platform's tenant settings
// with the side effects of a settings change registered.
type SettingsCallbacksTestModule struct {
	Settings       settingsCompose.SettingsService
	TenantSettings *settingsCompose.TenantOperations
}

// NewSettingsCallbacksTestModule composes the tenant settings over db and
// returns them with the realtime hub a settings change broadcasts on.
func NewSettingsCallbacksTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, unlinker peoplePorts.PhotoUnlinker) (SettingsCallbacksTestModule, *realtime.Hub, error) {
	module, _, err := NewOperatorSettingsTestModule(db, unit)
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
	RegisterStudentPhotoSettingsSideEffects(module.SettingsSideEffects, photos)
	operations := settingsCompose.NewTenantOperations(settings.Settings, settings.payroll, settings.runtime,
		module.SettingsSideEffects.Dispatch,
		func(_ context.Context, tenantID int64, key string) {
			_ = hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key}))
		})
	return SettingsCallbacksTestModule{Settings: settings.Settings, TenantSettings: operations}, hub, nil
}
