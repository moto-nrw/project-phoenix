package services

import (
	"log/slog"

	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type OperatorSettingsTestModule struct {
	ActiveTestModule
	CareLifecycle       users.CareLifecycleService
	SettingsSideEffects *sideeffects.Registry
}

func NewOperatorSettingsTestModule(db *bun.DB, unit tenant.UnitOfWork) (OperatorSettingsTestModule, error) {
	active, err := NewActiveTestModule(db, unit)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	registry := sideeffects.NewRegistry()
	wc := facilities.NewWCService(active.Facilities, facilitiesLegacy.ActivityCatalog(active.Activities), slog.Default())
	facilitiesLegacy.RegisterSettingsSideEffects(registry, active.Schulhof, wc)
	users.RegisterCareWithdrawalSettingsSideEffects(registry, care.CareLifecycle)
	return OperatorSettingsTestModule{ActiveTestModule: active, CareLifecycle: care.CareLifecycle, SettingsSideEffects: registry}, nil
}
