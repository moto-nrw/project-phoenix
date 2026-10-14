package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type StaffTestModule struct {
	WorkforceTestModule
	PeopleDirectory   peopledirectory.Capability
	membershipRuntime func(transactionruntime.Database, *slog.Logger, services.StaffMembershipHooks) services.StaffMembershipRuntime
}

func (m StaffTestModule) NewStaffMembershipRuntime(db transactionruntime.Database, logger *slog.Logger, hooks services.StaffMembershipHooks) services.StaffMembershipRuntime {
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
	people, err := services.NewPeopleDirectory(db.Pool())
	if err != nil {
		return StaffTestModule{}, err
	}
	carrier := &services.Factory{Users: work.Users, Education: groups.Education, WorkSession: work.TimeTracking}
	membershipRuntime := func(db transactionruntime.Database, logger *slog.Logger, hooks services.StaffMembershipHooks) services.StaffMembershipRuntime {
		return carrier.NewStaffMembershipRuntime(db.Pool(), auth.Auth, logger, hooks)
	}
	return StaffTestModule{WorkforceTestModule: work, PeopleDirectory: people, membershipRuntime: membershipRuntime}, nil
}
