package repositories

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
)

// NewCurrentAccountAccess binds the account holder's display and self-edits
// without exposing credentials or an account repository to user context.
func NewCurrentAccountAccess(profiles identityaccess.AccountProfiles) usercontext.AccountAccess {
	return currentAccountAccess{profiles}
}

type currentAccountAccess struct{ identityaccess.AccountProfiles }

// AccountExists supplies the People Directory's account-reference check.
// Only a missing account is absence; any other lookup failure is retained,
// rather than treating a failed read as absence.
func AccountExists(profiles identityaccess.AccountProfiles) func(context.Context, int64) (bool, error) {
	return func(ctx context.Context, accountID int64) (bool, error) {
		_, err := profiles.FindAccountMetadata(ctx, accountID)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return false, nil
		}
		return err == nil, err
	}
}

func (a currentAccountAccess) FindCurrentAccount(ctx context.Context, accountID int64) (*users.PersonAccount, error) {
	value, err := a.FindAccountMetadata(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := users.PersonAccount(value)
	return &result, nil
}
