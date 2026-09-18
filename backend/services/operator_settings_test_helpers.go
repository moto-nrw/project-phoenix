package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type OperatorSettingsTestModule struct {
	ActiveTestModule
	CareLifecycle       carelifecycle.CareLifecycleService
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
	carelifecycle.RegisterCareWithdrawalSettingsSideEffects(registry, care.CareLifecycle)
	return OperatorSettingsTestModule{ActiveTestModule: active, CareLifecycle: care.CareLifecycle, SettingsSideEffects: registry}, nil
}
