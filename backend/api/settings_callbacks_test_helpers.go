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

type SettingsCallbacksTestModule struct {
	Settings       settingsCompose.SettingsService
	TenantSettings *settingsCompose.TenantOperations
	RealtimeHub    *realtime.Hub
}

func NewSettingsCallbacksTestModule(db transactionruntime.Database, unit tenant.UnitOfWork, unlinker peoplePorts.PhotoUnlinker) (SettingsCallbacksTestModule, error) {
	module, err := NewOperatorSettingsTestModule(db, unit)
	if err != nil {
		return SettingsCallbacksTestModule{}, err
	}
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return SettingsCallbacksTestModule{}, err
	}
	hub := deliveryCompose.NewRealtimeHub(slog.Default())
	photoRuntime := new(peopleCompose.StudentPhotoRuntime)
	directory, err := peopleCompose.New(peopleCompose.Dependencies{
		StudentDirectory: studentDirectoryView.NewReader(),
		DB:               db.Pool(), Observe: func(peopleCompose.Observation) {},
		StudentPhotos: studentPhotoSlot{slot: photoRuntime},
	})
	if err != nil {
		return SettingsCallbacksTestModule{}, err
	}
	photos := NewStudentPhotos(directory, photoRuntime, StudentPhotoRuntimeDependencies{
		Settings: settings.Settings, Broadcaster: hub, Unlinker: unlinker, Logger: slog.Default(),
		ActingAccount: actingAccountID,
	})
	RegisterStudentPhotoSettingsSideEffects(module.SettingsSideEffects, photos)
	operations := settingsCompose.NewTenantOperations(settings.Settings, settings.payroll, settings.runtime,
		module.SettingsSideEffects.Dispatch,
		func(_ context.Context, tenantID int64, key string) {
			_ = hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key}))
		})
	return SettingsCallbacksTestModule{Settings: settings.Settings, TenantSettings: operations, RealtimeHub: hub}, nil
}
