package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/uptrace/bun"
)

// Identity & Access owns auth.accounts, platform.operators and the operator
// refresh sessions. The legacy repositories that used to join or select
// those tables declare the narrow fact they need and the factory binds it
// at construction to the owner (#2720): the parent dashboard reads the
// account's address through the public capability, the People Directory
// repositories filter or attach accounts through the owner's queries, and
// the retained operator repository contracts are adapters over the module.

// IdentityAccessObserver records one Identity & Access operation. It matches
// the root observer so the serving root can pass its metrics sink without
// depending on the module's composition package.
type IdentityAccessObserver func(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error)

// newIdentityAccess composes the Identity & Access module the factory's
// adapters read through. A nil observer composes it unobserved, which is
// what repository tests and CLI roots use.
func newIdentityAccess(db *bun.DB, observe IdentityAccessObserver) *identityaccess.Module {
	observer := func(identityCompose.Observation) {}
	if observe != nil {
		observer = func(observation identityCompose.Observation) {
			observe(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.StatementDuration, identityaccess.ErrorCode(observation.Err), observation.Err)
		}
	}
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: observer})
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose identity access: %v", err))
	}
	return module
}

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

// NewGuardianProfileRepository composes the People Directory guardian profile
// repository with the Identity & Access active-account (#2720),
// active-membership and guardian-role (#2721) queries its portal
// reachability check joins. Compositions that do not build the whole factory
// use it so no call site can forget the owner queries.
func NewGuardianProfileRepository(db *bun.DB) userModels.GuardianProfileRepository {
	return usersRepo.NewGuardianProfileRepository(db,
		usersRepo.WithPortalMemberships(newIdentityAccess(db, nil).FindActiveGuardianMemberships),
	)
}

// staffMessageIdentity returns the Identity & Access owner queries the staff
// messaging reads filter through.
func staffMessageIdentity(accounts identityaccess.StaffAccountQueries) usersRepo.StaffMessageIdentity {
	return usersRepo.StaffMessageIdentity{
		ActiveSchoolAccounts: accounts.ListActiveAccountIDsForTenant,
		RoleClasses:          schoolRoleClassQuery(accounts),
	}
}

// schoolRoleClassQuery adapts the owner's role classification to the staff
// messaging projection (#2721).
func schoolRoleClassQuery(roles identityaccess.StaffAccountQueries) usersRepo.SchoolRoleClassQuery {
	return func(ctx context.Context, tenantID int64, accountIDs []int64) ([]usersRepo.SchoolRoleClass, error) {
		rows, err := roles.ClassifySchoolRoles(ctx, tenantID, accountIDs)
		if err != nil {
			return nil, err
		}
		result := make([]usersRepo.SchoolRoleClass, 0, len(rows))
		for _, row := range rows {
			result = append(result, usersRepo.SchoolRoleClass{AccountID: row.AccountID, IsAdmin: row.IsAdmin, IsLehrkraft: row.IsLehrkraft})
		}
		return result, nil
	}
}

// NewPersonRepository composes the People Directory person repository with
// the Identity & Access account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db *bun.DB) userModels.PersonRepository {
	return usersRepo.NewPersonRepository(db, usersRepo.WithAccountLookup(accountLookup(newIdentityAccess(db, nil))))
}

// accountLookup adapts the owner's by-id read to the People Directory
// lookup: a missing account resolves to nil, as the former LEFT JOIN did.
func accountLookup(accounts identityaccess.AccountProfiles) usersRepo.AccountLookup {
	return func(ctx context.Context, accountID int64) (*userModels.PersonAccount, error) {
		account, err := accounts.FindAccountMetadata(ctx, accountID)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		result := userModels.PersonAccount(account)
		return &result, nil
	}
}
