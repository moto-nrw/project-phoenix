package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type SettingsCallbacksTestModule struct {
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
	parents, err := repositories.NewParentRouteTestRepositories(db)
	if err != nil {
		return SettingsCallbacksTestModule{}, err
	}
	hub := deliveryCompose.NewRealtimeHub(slog.Default())
	photos := users.NewStudentPhotoService(users.StudentPhotoServiceDependencies{
		StudentRepo: parents.Student, Settings: settings.Settings, UserContext: module.UserContext,
		Broadcaster: hub, Unlinker: unlinker, DB: db, Logger: slog.Default(),
	})
	users.RegisterStudentPhotoSettingsSideEffects(module.SettingsSideEffects, photos)
	operations := config.NewTenantOperations(settings.Settings, settings.payroll, settings.runtime,
		settings.homeLayouts,
		module.SettingsSideEffects.Dispatch,
		func(_ context.Context, tenantID int64, key string) {
			_ = hub.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventTenantSettingsChanged, "", realtime.EventData{Source: &key}))
		})
	return SettingsCallbacksTestModule{TenantSettings: operations, RealtimeHub: hub}, nil
}
