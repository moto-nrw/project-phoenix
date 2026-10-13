package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// Identity & Access owns auth.accounts, platform.operators and the operator
// refresh sessions. The legacy repositories that used to join or select
// those tables declare the narrow fact they need and the factory binds it
// at construction to the owner (#2720): the parent dashboard reads the
// account's address through the public capability, the People Directory
// repositories filter or attach accounts through the owner's queries, and
// the retained operator repository contracts are adapters over the module.

// identityAccountDirectory adapts the public account lookup to the parent
// enrollment repository's directory port.
type identityAccountDirectory struct {
	accounts identityaccess.GuardianAccessQuery
}

// AccountEmail resolves the address behind an account; an unknown account is
// an empty address, matching the applications by id only.
func (d identityAccountDirectory) AccountEmail(ctx context.Context, accountID int64) (string, error) {
	account, err := d.accounts.FindAccount(ctx, accountID)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return account.Email, nil
}

// staffMessageIdentity returns the Identity & Access owner queries the staff
// messaging reads filter through, including the role classification
// (#2721).
func staffMessageIdentity(accounts identityaccess.StaffAccountQueries) peopleCompose.StaffMessageIdentity {
	return peopleCompose.StaffMessageIdentity{
		ActiveSchoolAccounts: accounts.ListActiveAccountIDsForTenant,
		RoleClasses: func(ctx context.Context, tenantID int64, accountIDs []int64) ([]peopleCompose.StaffMessageRole, error) {
			rows, err := accounts.ClassifySchoolRoles(ctx, tenantID, accountIDs)
			if err != nil {
				return nil, err
			}
			result := make([]peopleCompose.StaffMessageRole, 0, len(rows))
			for _, row := range rows {
				result = append(result, peopleCompose.StaffMessageRole{AccountID: row.AccountID, IsAdmin: row.IsAdmin, IsLehrkraft: row.IsLehrkraft})
			}
			return result, nil
		},
	}
}
