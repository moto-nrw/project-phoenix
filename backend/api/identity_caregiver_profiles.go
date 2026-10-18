package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// Identity & Access owns role and permission management (#3314). This file
// holds the caregiver-profile walk the module's staff seam reads, over the
// People Directory and School Membership.

// CaregiverProfiles answers whether an account holds a live teacher profile
// at the tenant in ctx, through the School Membership staff identity read
// over the People Directory. Soft-deleted (offboarded) records do not count.
// Every path that guards the Lehrkraft role reads this one walk (#1772): the
// Identity & Access role administration and school identity seam as well as
// the operator school access.
type CaregiverProfiles struct {
	Persons    caregiverPersons
	Membership schoolmembership.StaffIdentities
}

// caregiverPersons is the People Directory read behind an account's person.
type caregiverPersons interface {
	FindPersonByAccount(ctx context.Context, accountID int64) (peopledirectory.Person, error)
}

// liveCaregiverProfile is the Lehrkraft guard the staff seams read through
// CaregiverProfiles.
type liveCaregiverProfile interface {
	hasLive(ctx context.Context, accountID int64) (bool, error)
}

func (c CaregiverProfiles) complete() bool { return c.Persons != nil && c.Membership != nil }

func (c CaregiverProfiles) hasLive(ctx context.Context, accountID int64) (bool, error) {
	identity, err := c.Membership.ResolveStaffIdentityByAccount(ctx, accountID, accountPersons{directory: c.Persons})
	if err != nil {
		return false, err
	}
	return identity.IsTeacher(), nil
}

// accountPersons serves School Membership's consumer-owned person port over
// the People Directory; a missing person is the clean "no person" outcome.
type accountPersons struct{ directory caregiverPersons }

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
