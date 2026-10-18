//go:build testhooks

package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsOperatorAPI "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OperatorSettingsTestModule is the Settings side of the operator settings
// routes: the tenant settings, the write side effects and the root's
// optional dependencies of the operator's school settings. The care
// lifecycle the side effects drive comes beside it from
// NewOperatorSettingsTestModule (#4080).
type OperatorSettingsTestModule struct {
	Settings            settingsCompose.BatchSettingsService
	SettingsSideEffects configModels.SideEffectDispatcher
	schoolSlugs         settingsOperatorAPI.SchoolLookup
	// operator are the root's required dependencies of the operator's school
	// settings; OperatorSchoolSettingsWith adds the optional ones.
	operator settingsCompose.OperatorDependencies
}

// NewOperatorSettingsTestModule composes the operator settings routes over
// the active graph and returns them with the Care Plan lifecycle the side
// effects drive.
func NewOperatorSettingsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (OperatorSettingsTestModule, careplan.CareLifecycle, error) {
	settings := NewActiveTestSettings(db, unit)
	presence := NewActiveTestPresence(db, unit)
	effects, careLifecycle, err := newOperatorSettingSideEffects(db, unit)
	if err != nil {
		return OperatorSettingsTestModule{}, nil, err
	}
	schools, err := NewOrganizationTenancy(db)
	if err != nil {
		return OperatorSettingsTestModule{}, nil, err
	}
	registry, err := settingsCompose.NewSideEffects(func(string) {}, effects...)
	if err != nil {
		return OperatorSettingsTestModule{}, nil, err
	}
	return OperatorSettingsTestModule{
		Settings:            settings.Settings,
		SettingsSideEffects: registry,
		schoolSlugs:         operatorSettingsSchools{schools: schools},
		operator: settingsCompose.OperatorDependencies{
			Settings: settings.Settings, DB: db.Pool(), OpenAttendance: OpenAttendanceChecker{Service: presence.Active},
		},
	}, careLifecycle, nil
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
	return m.schoolSlugs
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
	deps := m.operator
	deps.Notify = opts.Notify
	deps.OnValueSet = opts.OnValueSet
	return settingsCompose.NewOperatorSchoolSettings(deps)
}

// newOperatorSettingSideEffects are the checkout switches of Facilities and
// the booking authority of Care Plan over the active graph, as the serving
// root binds them, with the Care Plan lifecycle they drive.
func newOperatorSettingSideEffects(db transactionruntime.Database, unit tenant.UnitOfWork) ([]settingsCompose.SettingSideEffect, careplan.CareLifecycle, error) {
	facilities := NewActiveTestFacilities(db, unit)
	timetableRows := NewActiveTestTimetable(db, unit)
	care, _, err := NewCareLifecycleTestModule(db, unit)
	if err != nil {
		return nil, nil, err
	}
	wc := facilitiesCompose.NewWCService(facilities.Facilities, FacilitiesActivityCatalog(timetableRows.Activities), slog.Default())
	schulhof, wcSwitch := facilitiesCompose.SettingHandlers(facilities.SchulhofService, wc)
	return []settingsCompose.SettingSideEffect{
		{Key: configModels.KeyCheckoutSchulhofEnabled, Handle: schulhof},
		{Key: configModels.KeyCheckoutWCEnabled, Handle: wcSwitch},
		careWithdrawalSettingSideEffect(care.CareLifecycle),
	}, care.CareLifecycle, nil
}
