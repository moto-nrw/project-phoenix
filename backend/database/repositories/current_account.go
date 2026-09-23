package repositories

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

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
