package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SettingsTestModule contains only the settings application capabilities.
type SettingsTestModule struct {
	Settings       config.SettingsService
	TenantSettings *config.TenantOperations
	payroll        config.PayrollStatusGetter
	runtime        config.TenantOperationsRuntime
}

// NewSettingsTestModule retains the real settings storage, guards and tenant
// runtime without composing attendance, auth, or other route services.
func NewSettingsTestModule(db *bun.DB, unit tenant.UnitOfWork) (SettingsTestModule, error) {
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return SettingsTestModule{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return SettingsTestModule{}, err
	}
	runtime := newSettingsRuntime(db, &unit).WithSchoolMembership(membership)
	repos := repositories.NewSettingsTestRepositories(db, runtime)
	settings := config.NewSettingsService(
		repos.Values, repos.Audit,
		newSchoolSettingsStore(organizations), runtime, slog.Default(),
	)
	guards := settings.(interface {
		SetClassRestrictionGuard(func(context.Context) (bool, error))
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
		SetGradeCapGuard(func(context.Context) (int, error))
	})
	guards.SetClassRestrictionGuard(repos.ClassRestriction)
	guards.SetGradeRestrictionGuard(repos.GradeRestriction)
	guards.SetGradeCapGuard(repos.GradeCap)
	payroll := config.NewPayrollStatusService(settings, func(ctx context.Context) (int, int, error) {
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
		TenantSettings: config.NewTenantOperations(settings, payroll, runtime, nil, nil),
	}, nil
}
