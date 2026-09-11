package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
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
// repository with the Identity & Access active-account query its portal
// reachability check joins (#2720). Compositions that do not build the whole
// factory use it so no call site can forget the owner query.
func NewGuardianProfileRepository(db *bun.DB) userModels.GuardianProfileRepository {
	return usersRepo.NewGuardianProfileRepository(db, usersRepo.WithActiveAccounts(activeAccountQuery(mustAccountRepository(authRepo.NewAccountRepository(db)))))
}

// NewPersonRepository composes the People Directory person repository with
// the Identity & Access account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db *bun.DB) userModels.PersonRepository {
	return usersRepo.NewPersonRepository(db, usersRepo.WithAccountLookup(accountLookup(authRepo.NewAccountRepository(db))))
}

// activeAccountQuery returns the owner query the People Directory
// repositories join.
func activeAccountQuery(accounts *authRepo.AccountRepository) usersRepo.ActiveAccountQuery {
	return func(ctx context.Context) *bun.SelectQuery { return accounts.ActiveAccountIDs(ctx) }
}

// accountLookup adapts the owner's by-id read to the People Directory
// lookup: a missing account resolves to nil, as the former LEFT JOIN did.
func accountLookup(accounts authModels.AccountRepository) usersRepo.AccountLookup {
	return func(ctx context.Context, accountID int64) (*authModels.Account, error) {
		account, err := accounts.FindByID(ctx, accountID)
		if authRepo.IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return account, nil
	}
}

// mustAccountRepository returns the concrete Identity & Access account
// repository whose owner queries the People Directory bindings consume.
func mustAccountRepository(repository authModels.AccountRepository) *authRepo.AccountRepository {
	accounts, ok := repository.(*authRepo.AccountRepository)
	if !ok {
		panic("repository factory: account repository must be the Identity & Access Postgres adapter")
	}
	return accounts
}
