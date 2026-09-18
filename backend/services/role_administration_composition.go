package services

import (
	"context"
	"database/sql"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Identity & Access owns role and permission management (#3314): role and
// permission CRUD, account role assignment with its identity side effects,
// direct account grants and role-permission selections. The module's role
// storage seam is bound to the retained repositories in
// database/repositories until #3226 moves them into the module; this file
// holds the caregiver-profile walk the module's staff seam reads and serves
// the consumer-owned ports of the retained auth and people services over the
// public module.

// hasLiveCaregiverProfile walks the person → staff → teacher chain of the
// account at the tenant in ctx. Soft-deleted (offboarded) records do not
// count. Every path that guards the Lehrkraft role reads this one walk
// (#1772): the Identity & Access role administration and school identity
// seam as well as the operator school access.
func hasLiveCaregiverProfile(
	ctx context.Context,
	persons userModels.PersonRepository,
	staffs userModels.StaffRepository,
	teachers userModels.TeacherRepository,
	accountID int64,
) (bool, error) {
	person, err := persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}
	if person == nil || person.DeletedAt != nil {
		return false, nil
	}
	staff, err := staffs.FindByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if staff == nil || staff.DeletedAt != nil {
		return false, nil
	}
	teacher, err := teachers.FindByStaffID(ctx, staff.ID)
	if err != nil {
		return false, err
	}
	return teacher != nil && teacher.DeletedAt == nil, nil
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
