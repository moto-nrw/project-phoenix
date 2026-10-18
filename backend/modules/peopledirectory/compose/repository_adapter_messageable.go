package compose

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/peoplepostgres"
	"github.com/uptrace/bun"
)

// NewMessageableGuardians is the parent-message recipient lookup over the
// retained People Directory rows; memberships bounds it to the guardians'
// active schools (Identity & Access).
func NewMessageableGuardians(db *bun.DB, links usersRepo.GuardianLinkRows, memberships func(context.Context, []int64, []int64) (map[int64][]int64, error)) *usersRepo.MessageableGuardianRepository {
	return usersRepo.NewMessageableGuardianRepository(NewLegacyRepositoryRuntime(db), links, memberships)
}

// StaffMessageRole is the school role class of one account.
type StaffMessageRole struct {
	AccountID   int64
	IsAdmin     bool
	IsLehrkraft bool
}

// StaffMessageIdentity are the Identity & Access facts the colleague lookup
// filters through.
type StaffMessageIdentity struct {
	// ActiveSchoolAccounts narrows candidates to globally active accounts
	// with an active membership at the given school.
	ActiveSchoolAccounts func(context.Context, int64, []int64) ([]int64, error)
	// RoleClasses classifies the roles accounts hold at a school.
	RoleClasses func(context.Context, int64, []int64) ([]StaffMessageRole, error)
}

// NewMessageableStaff is the staff-messaging colleague lookup over the
// retained People Directory rows: staffAccounts lists the live staff
// accounts of the caller's school (School Membership), identity the account
// facts.
func NewMessageableStaff(db *bun.DB, staffAccounts func(context.Context) ([]int64, error), identity StaffMessageIdentity) *usersRepo.MessageableStaffRepository {
	return usersRepo.NewMessageableStaffRepository(NewLegacyRepositoryRuntime(db), staffAccounts, usersRepo.StaffMessageIdentity{
		ActiveSchoolAccounts: identity.ActiveSchoolAccounts,
		RoleClasses: func(ctx context.Context, tenantID int64, accountIDs []int64) ([]usersRepo.SchoolRoleClass, error) {
			roles, err := identity.RoleClasses(ctx, tenantID, accountIDs)
			if err != nil {
				return nil, err
			}
			result := make([]usersRepo.SchoolRoleClass, 0, len(roles))
			for _, role := range roles {
				result = append(result, usersRepo.SchoolRoleClass(role))
			}
			return result, nil
		},
	})
}
