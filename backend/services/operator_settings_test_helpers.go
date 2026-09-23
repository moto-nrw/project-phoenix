package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type OperatorSettingsTestModule struct {
	ActiveTestModule
	CareLifecycle       careplan.CareLifecycle
	SettingsSideEffects *sideeffects.Registry
	// CareWithdrawals stores the pending withdrawal tasks a booking
	// authority test starts from.
	CareWithdrawals userModels.CareWithdrawalCompletionRepository
	// Schools is the Organisation & Tenancy owner the slug lookup reads.
	Schools organizationtenancy.Capability
	db      *bun.DB
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
	careRepos, err := repositories.NewCareLifecycleTestRepositories(db, repositories.NewTestAuditStore(db))
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	schools, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	registry := sideeffects.NewRegistry()
	wc := facilities.NewWCService(active.Facilities, facilitiesLegacy.ActivityCatalog(active.Activities), slog.Default())
	facilitiesLegacy.RegisterSettingsSideEffects(registry, active.Schulhof, wc)
	registerCareWithdrawalSettingsSideEffects(registry, care.CareLifecycle)
	return OperatorSettingsTestModule{
		ActiveTestModule:    active,
		CareLifecycle:       care.CareLifecycle,
		SettingsSideEffects: registry,
		CareWithdrawals:     careRepos.CareWithdrawal,
		Schools:             schools,
		db:                  db,
	}, nil
}

// OperatorSchoolSettings composes the operator's school settings the way
// api/base.go does, over this module's services: the presence-mode guard
// reads the module's Student Presence and the booking authority preview its
// Care Plan. onValueSet is the write side-effect hook; the broadcast is off.
func (m OperatorSettingsTestModule) OperatorSchoolSettings(onValueSet config.OperatorValueSetHook) settings.OperatorSchoolSettings {
	return m.OperatorSchoolSettingsWith(OperatorSchoolSettingsTestOptions{OnValueSet: onValueSet})
}

// OperatorSchoolSettingsTestOptions varies the root's optional dependencies
// of the operator's school settings.
type OperatorSchoolSettingsTestOptions struct {
	OnValueSet config.OperatorValueSetHook
	// Notify receives the broadcast; nil disables it.
	Notify config.SettingsChangedNotifier
	// WithoutCarePlan composes a deployment without the Care Plan booking
	// authority.
	WithoutCarePlan bool
}

// OperatorSchoolSettingsWith composes the operator's school settings over
// this module's services with the given optional dependencies.
func (m OperatorSettingsTestModule) OperatorSchoolSettingsWith(opts OperatorSchoolSettingsTestOptions) settings.OperatorSchoolSettings {
	deps := settingsCompose.OperatorDependencies{
		Settings:       m.Settings,
		DB:             m.db,
		Notify:         opts.Notify,
		OpenAttendance: openAttendanceChecker{service: m.Active},
		CareLifecycle:  m.CareLifecycle,
		OnValueSet:     opts.OnValueSet,
	}
	if opts.WithoutCarePlan {
		deps.CareLifecycle = nil
	}
	return settingsCompose.NewOperatorSchoolSettings(deps)
}
