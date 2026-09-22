package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// Identity & Access owns role and permission management (#3314): role and
// permission CRUD, account role assignment with its identity side effects,
// direct account grants and role-permission selections. The module's role
// storage seam is bound to the retained repositories in
// database/repositories until #3226 moves them into the module; this file
// holds the caregiver-profile walk the module's staff seam reads and serves
// the consumer-owned ports of the retained auth and people services over the
// public module.

// caregiverProfiles answers whether an account holds a live teacher profile
// at the tenant in ctx, through the School Membership staff identity read
// over the People Directory. Soft-deleted (offboarded) records do not count.
// Every path that guards the Lehrkraft role reads this one walk (#1772): the
// Identity & Access role administration and school identity seam as well as
// the operator school access.
type caregiverProfiles struct {
	persons    peopledirectory.Query
	membership schoolmembership.StaffIdentities
}

func (c caregiverProfiles) complete() bool { return c.persons != nil && c.membership != nil }

func (c caregiverProfiles) hasLive(ctx context.Context, accountID int64) (bool, error) {
	identity, err := c.membership.ResolveStaffIdentityByAccount(ctx, accountID, accountPersons{directory: c.persons})
	if err != nil {
		return false, err
	}
	return identity.IsTeacher(), nil
}

// accountPersons serves School Membership's consumer-owned person port over
// the People Directory; a missing person is the clean "no person" outcome.
type accountPersons struct{ directory peopledirectory.Query }

func (p accountPersons) FindPersonIDByAccount(ctx context.Context, accountID int64) (int64, bool, error) {
	person, err := p.directory.FindPersonByAccount(ctx, accountID)
	if errors.Is(err, peopledirectory.ErrPersonNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return person.ID, true, nil
}

// roleAdministration serves the people services' role port over the public
// module. The module is composed after the person service, so it is read at
// call time.
type roleAdministration struct {
	current func() *identityaccess.Module
}

func (r roleAdministration) AssignRoleToAccount(ctx context.Context, accountID, roleID int64) error {
	module := r.current()
	if module == nil {
		return identityaccess.ErrAccountLifecycleUnavailable
	}
	return module.AssignRoleToAccount(ctx, accountID, roleID)
}

func (r roleAdministration) RemoveRoleFromAccount(ctx context.Context, accountID, roleID int64) error {
	module := r.current()
	if module == nil {
		return identityaccess.ErrAccountLifecycleUnavailable
	}
	return module.RemoveRoleFromAccount(ctx, accountID, roleID)
}

func (r roleAdministration) AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error) {
	module := r.current()
	if module == nil {
		return false, identityaccess.ErrAccountLifecycleUnavailable
	}
	return module.AccountHoldsLehrkraftRole(ctx, accountID)
}
