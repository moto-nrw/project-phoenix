//go:build testhooks

package api

import (
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StaffTestModule is the People Directory side of the staff routes: the
// person directory and the owner capability, with the staff membership
// runtime composed over them.
type StaffTestModule struct {
	Users             peoplePorts.PersonDirectory
	PeopleDirectory   peopledirectory.Capability
	membershipRuntime func(transactionruntime.Database, *slog.Logger, StaffMembershipHooks) StaffMembershipRuntime
}

func (m StaffTestModule) NewStaffMembershipRuntime(db transactionruntime.Database, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	return m.membershipRuntime(db, logger, hooks)
}

// NewStaffTestModule composes the staff routes' People Directory side and
// returns it with the Workforce owner the routes administer staff through.
func NewStaffTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (StaffTestModule, WorkforceTestModule, error) {
	work, users, err := NewWorkforceTestModule(db, unit)
	if err != nil {
		return StaffTestModule{}, WorkforceTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return StaffTestModule{}, WorkforceTestModule{}, err
	}
	groups, _, _, _, err := NewGroupsTestModule(db, unit)
	if err != nil {
		return StaffTestModule{}, WorkforceTestModule{}, err
	}
	people, err := owners.NewPeopleDirectory(db)
	if err != nil {
		return StaffTestModule{}, WorkforceTestModule{}, err
	}
	membershipRuntime := func(db transactionruntime.Database, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
		return NewStaffMembershipRuntime(users, groups.Education, work.TimeTracking.StaffDay, nil, db, auth.Auth, logger, hooks)
	}
	return StaffTestModule{Users: users, PeopleDirectory: people, membershipRuntime: membershipRuntime}, work, nil
}
