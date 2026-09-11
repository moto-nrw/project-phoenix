package services

import (
	"log/slog"

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
	carrier := &Factory{Users: work.Users, Auth: auth.Auth, Education: groups.Education, StaffAbsence: work.StaffAbsence, WorkSession: work.WorkSession}
	return StaffTestModule{WorkforceTestModule: work, membershipRuntime: carrier.NewStaffMembershipRuntime}, nil
}
