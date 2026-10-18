//go:build testhooks

package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsOperatorAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type OperatorSettingsTestModule struct {
	ActiveTestModule
	CareLifecycle       careplan.CareLifecycle
	SettingsSideEffects *settingsCompose.SideEffectRegistry
	// CareWithdrawals stores the pending withdrawal tasks a booking
	// authority test starts from.
	CareWithdrawals userModels.CareWithdrawalCompletionRepository
	// Schools is the Organisation & Tenancy owner the slug lookup reads.
	Schools organizationtenancy.Capability
	db      transactionruntime.Database
}

func NewOperatorSettingsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (OperatorSettingsTestModule, error) {
	active, err := NewActiveTestModule(db, unit)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	care, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	careRepos, err := NewCareLifecycleTestRepositories(db, NewTestAuditStore(db))
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	schools, err := NewOrganizationTenancy(db)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
	registry, err := settingsCompose.NewSideEffects(func(string) {}, operatorSettingSideEffects(active, care.CareLifecycle)...)
	if err != nil {
		return OperatorSettingsTestModule{}, err
	}
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
// reads the module's Student Presence. onValueSet is the write side-effect
// hook; the broadcast is off.
func (m OperatorSettingsTestModule) OperatorSchoolSettings(onValueSet settingsCompose.OperatorValueSetHook) settings.OperatorSchoolSettings {
	return m.OperatorSchoolSettingsWith(OperatorSchoolSettingsTestOptions{OnValueSet: onValueSet})
}

// SchoolSlugs binds the operator settings routes' slug lookup to this
// module's Organisation & Tenancy, the way the root does.
func (m OperatorSettingsTestModule) SchoolSlugs() settingsOperatorAPI.SchoolLookup {
	return operatorSettingsSchools{schools: m.Schools}
}

// OperatorSchoolSettingsTestOptions varies the root's optional dependencies
// of the operator's school settings.
type OperatorSchoolSettingsTestOptions struct {
	OnValueSet settingsCompose.OperatorValueSetHook
	// Notify receives the broadcast; nil disables it.
	Notify settingsCompose.SettingsChangedNotifier
}

// OperatorSchoolSettingsWith composes the operator's school settings over
// this module's services with the given optional dependencies.
func (m OperatorSettingsTestModule) OperatorSchoolSettingsWith(opts OperatorSchoolSettingsTestOptions) settings.OperatorSchoolSettings {
	deps := settingsCompose.OperatorDependencies{
		Settings:       m.Settings,
		DB:             m.db.Pool(),
		Notify:         opts.Notify,
		OpenAttendance: OpenAttendanceChecker{Service: m.Active},
		OnValueSet:     opts.OnValueSet,
	}
	return settingsCompose.NewOperatorSchoolSettings(deps)
}

// operatorSettingSideEffects are the checkout switches of Facilities and the
// booking authority of Care Plan, as the serving root binds them.
func operatorSettingSideEffects(active ActiveTestModule, care careplan.BookingAuthority) []settingsCompose.SettingSideEffect {
	wc := facilitiesCompose.NewWCService(active.Facilities, FacilitiesActivityCatalog(active.Activities), slog.Default())
	schulhof, wcSwitch := facilitiesLegacy.SettingHandlers(active.Schulhof, wc)
	return []settingsCompose.SettingSideEffect{
		{Key: configModels.KeyCheckoutSchulhofEnabled, Handle: schulhof},
		{Key: configModels.KeyCheckoutWCEnabled, Handle: wcSwitch},
		careWithdrawalSettingSideEffect(care),
	}
}
