//go:build testhooks

package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type StaffTestModule struct {
	WorkforceTestModule
	PeopleDirectory   peopledirectory.Capability
	membershipRuntime func(transactionruntime.Database, *slog.Logger, StaffMembershipHooks) StaffMembershipRuntime
}

func (m StaffTestModule) NewStaffMembershipRuntime(db transactionruntime.Database, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	return m.membershipRuntime(db, logger, hooks)
}

func NewStaffTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (StaffTestModule, error) {
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
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return StaffTestModule{}, err
	}
	membershipRuntime := func(db transactionruntime.Database, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
		return NewStaffMembershipRuntime(work.Users, groups.Education, work.TimeTracking.StaffDay, nil, db, auth.Auth, logger, hooks)
	}
	return StaffTestModule{WorkforceTestModule: work, PeopleDirectory: people, membershipRuntime: membershipRuntime}, nil
}
