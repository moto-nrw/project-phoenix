package services

import (
	"context"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type SettingsCallbacksTestModule struct {
	Settings       config.SettingsService
	TenantSettings *config.TenantOperations
	RealtimeHub    *realtime.Hub
}

func NewSettingsCallbacksTestModule(db *bun.DB, unit tenant.UnitOfWork, unlinker users.PhotoUnlinker) (SettingsCallbacksTestModule, error) {
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
		DB: db, Observe: func(peopleCompose.Observation) {},
		StudentPhotoRuntime: func() peopleCompose.StudentPhotoRuntime { return *photoRuntime },
	})
	if err != nil {
		return SettingsCallbacksTestModule{}, err
	}
	photos := NewStudentPhotos(directory, photoRuntime, StudentPhotoRuntimeDependencies{
		Settings: settings.Settings, Broadcaster: hub, Unlinker: unlinker, Logger: slog.Default(),
	})
	users.RegisterStudentPhotoSettingsSideEffects(module.SettingsSideEffects, photos)
	operations := config.NewTenantOperations(settings.Settings, settings.payroll, settings.runtime,
		module.SettingsSideEffects.Dispatch,
		func(_ context.Context, tenantID int64, key string) {
			_ = hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key}))
		})
	return SettingsCallbacksTestModule{Settings: settings.Settings, TenantSettings: operations, RealtimeHub: hub}, nil
}
