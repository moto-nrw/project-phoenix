package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type StaffTestModule struct {
	WorkforceTestModule
	membershipRuntime func(*bun.DB, *slog.Logger, StaffMembershipHooks) StaffMembershipRuntime
}

func (m StaffTestModule) NewStaffMembershipRuntime(db *bun.DB, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	return m.membershipRuntime(db, logger, hooks)
}

func NewStaffTestModule(db *bun.DB, unit tenant.UnitOfWork) (StaffTestModule, error) {
	work, err := NewWorkforceTestModule(db, unit)
	if err != nil {
		return StaffTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return StaffTestModule{}, err
	}
	groups, err := NewGroupsTestModule(db, unit)
	if err != nil {
		return StaffTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return StaffTestModule{}, err
	}
	r, err := repositories.NewWorkforceTestRepositories(db, command)
	if err != nil {
		return StaffTestModule{}, err
	}
	a, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return StaffTestModule{}, err
	}
	u, err := repositories.NewUserContextTestRepositories(db)
	if err != nil {
		return StaffTestModule{}, err
	}
	offboarding := users.NewStaffOffboardingService(users.StaffOffboardingServiceDependencies{
		PersonRepo: r.Person, StaffRepo: r.Staff, TeacherRepo: r.Teacher, GroupSupervisorRepo: r.GroupSupervisor,
		GroupTeacherRepo: r.GroupTeacher, ClassTeacherRepo: r.ClassTeacher, GroupSubstitutionRepo: u.Substitutions,
		ActivitySupervisorRepo: r.ActivitySupervisor, InstanceStaffRepo: r.InstanceStaff, StaffShiftRepo: r.StaffShift,
		StaffShiftSeriesRepo: r.StaffShiftSeries, StaffAbsenceRepo: r.StaffAbsence, AccountRepo: a.Account,
		AccountTenantRepo: a.AccountTenant, RoleRepo: a.Role, AccountPermissionRepo: a.AccountPermission,
		DataDeletionRepo: r.DataDeletion, TimeTrackingDeleteRepo: r.TimeTrackingDeletion, AuthService: auth.Auth, DB: db, Logger: slog.Default(),
	})
	carrier := &Factory{Users: work.Users, Auth: auth.Auth, Education: groups.Education, StaffAbsence: work.StaffAbsence, WorkSession: work.WorkSession, StaffOffboarding: offboarding}
	return StaffTestModule{WorkforceTestModule: work, membershipRuntime: carrier.NewStaffMembershipRuntime}, nil
}
