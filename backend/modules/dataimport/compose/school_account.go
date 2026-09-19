package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// NewSchoolAccountLookup exposes only the identity facts the duplicate matcher
// needs. A missing or inactive mapping is an ordinary absence, not a failure.
func NewSchoolAccountLookup(accounts identityaccess.SchoolAccountQuery) func(context.Context, string) (int64, bool, error) {
	return func(ctx context.Context, email string) (int64, bool, error) {
		account, err := accounts.FindSchoolAccountByEmail(ctx, email)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return 0, false, nil
		}
		if err != nil {
			return 0, false, err
		}
		return account.ID, true, nil
	}
}
