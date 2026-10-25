//go:build testhooks

package owners

import (
	"context"
	"log/slog"
	"time"

	snapshotRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SettingsTestModule contains only the settings application capabilities.
type SettingsTestModule struct {
	Settings       settingsCompose.BatchSettingsService
	TenantSettings snapshotRecords.TenantOperationsService
	Payroll        settingsCompose.PayrollStatusGetter
	Runtime        settingsCompose.TenantOperationsRuntime
	// Traeger is the Träger level over the same store (ADR 0086).
	Traeger *settingsCompose.TraegerSettings
}

// NewSettingsTestModule retains the real settings storage, guards and tenant
// runtime without composing attendance, auth, or other route services.
func NewSettingsTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (SettingsTestModule, error) {
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return SettingsTestModule{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return SettingsTestModule{}, err
	}
	runtime := settingsCompose.NewTenantSettingsRuntime(db.Pool(), &unit)
	repos := NewSettingsTestRepositories(runtime)
	homeLayouts := settingsCompose.NewHomeLayoutService(settingsCompose.NewHomeLayoutRepository(runtime), runtime, slog.Default())
	settings, err := settingsCompose.NewSettings(settingsCompose.SettingsDependencies{
		Values: repos.Values, Audit: repos.Audit, Schools: NewSchoolSettingsStore(organizations), Runtime: runtime,
		Logger: slog.Default(), HomeLayouts: homeLayouts, ObserveLookup: func(string, string, string, time.Duration) {},
		ClassRestrictionGuard: repos.ClassRestriction, GradeRestrictionGuard: repos.GradeRestriction, GradeCapGuard: repos.GradeCap,
		TraegerValues: settingsCompose.NewTraegerSettingRepository(runtime), SchoolTraeger: schoolTraeger(organizations),
	})
	if err != nil {
		return SettingsTestModule{}, err
	}
	traeger, err := BuildTraegerSettings(db, unit, slog.Default(), organizations, nil)
	if err != nil {
		return SettingsTestModule{}, err
	}
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
		Payroll:        payroll,
		Runtime:        runtime,
		Traeger:        traeger,
		TenantSettings: settingsCompose.NewTenantOperations(settings, payroll, runtime, nil, nil),
	}, nil
}

// TenantSettingsWithHook is TenantSettings with the value-set side effect
// hook bound at construction.
func (m SettingsTestModule) TenantSettingsWithHook(hook func(context.Context, int64, string, any) (func(), error)) snapshotRecords.TenantOperationsService {
	return settingsCompose.NewTenantOperations(m.Settings, m.Payroll, m.Runtime, hook, nil)
}

// TenantSnapshots binds the Worker's minute-snapshot read over this module's
// settings service, the way the Serve root binds it (#2746).
func (m SettingsTestModule) TenantSnapshots() (snapshotRecords.TenantSnapshots, error) {
	return settingsCompose.NewTenantSnapshots(m.Settings)
}
