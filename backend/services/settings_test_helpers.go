package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SettingsTestModule contains only the settings application capabilities.
type SettingsTestModule struct {
	Settings       settingsCompose.BatchSettingsService
	TenantSettings *settingsCompose.TenantOperations
	payroll        settingsCompose.PayrollStatusGetter
	runtime        settingsCompose.TenantOperationsRuntime
}

// NewSettingsTestModule retains the real settings storage, guards and tenant
// runtime without composing attendance, auth, or other route services.
func NewSettingsTestModule(db *bun.DB, unit tenant.UnitOfWork) (SettingsTestModule, error) {
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return SettingsTestModule{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return SettingsTestModule{}, err
	}
	runtime := settingsCompose.NewTenantSettingsRuntime(db, &unit)
	repos := NewSettingsTestRepositories(runtime)
	homeLayouts := settingsCompose.NewHomeLayoutService(settingsCompose.NewHomeLayoutRepository(runtime), runtime, slog.Default())
	settings := settingsCompose.NewSettingsServiceWithHomeLayouts(
		repos.Values, repos.Audit,
		NewSchoolSettingsStore(organizations), runtime, slog.Default(), homeLayouts,
	)
	guards := settings.(interface {
		SetClassRestrictionGuard(func(context.Context) (bool, error))
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
		SetGradeCapGuard(func(context.Context) (int, error))
	})
	guards.SetClassRestrictionGuard(repos.ClassRestriction)
	guards.SetGradeRestrictionGuard(repos.GradeRestriction)
	guards.SetGradeCapGuard(repos.GradeCap)
	payroll := settingsCompose.NewPayrollStatusService(settings, func(ctx context.Context) (int, int, error) {
		staff, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{})
		if err != nil {
			return 0, 0, err
		}
		missing := 0
		for _, member := range staff {
			if member.PersonnelNumber == nil || *member.PersonnelNumber == "" {
				missing++
			}
		}
		return len(staff), missing, nil
	})
	return SettingsTestModule{
		Settings:       settings,
		payroll:        payroll,
		runtime:        runtime,
		TenantSettings: settingsCompose.NewTenantOperations(settings, payroll, runtime, nil, nil),
	}, nil
}

// TenantSnapshots binds the Worker's minute-snapshot read over this module's
// settings service, the way the Serve root binds it (#2746).
func (m SettingsTestModule) TenantSnapshots() (settingsCompose.TenantSnapshots, error) {
	return settingsCompose.NewTenantSnapshots(m.Settings)
}
