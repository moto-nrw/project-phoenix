package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

var errMembershipLockOutsideTransaction = errors.New("the school membership lock needs the caller's school transaction")

// NewSchoolMembershipLock binds the membership lock of #6529 to the store,
// independently of the full composition. A lock outside a school transaction
// would end with its own statement, so it is refused.
func NewSchoolMembershipLock(store ports.SchoolMembershipLockStore, tenantOf func(context.Context) int64, transaction func(context.Context) (any, bool)) identityaccess.SchoolMembershipLock {
	return func(ctx context.Context, accountID int64) (bool, error) {
		if accountID <= 0 {
			return false, nil
		}
		tenantID := tenantOf(ctx)
		if tenantID <= 0 {
			return false, failed("lock school membership", domain.ErrTenantRequired)
		}
		if _, open := transaction(ctx); !open {
			return false, failed("lock school membership", errMembershipLockOutsideTransaction)
		}
		// Operator revocation locks account before mapping. Take KEY SHARE
		// first so a later audit FK check cannot deadlock with that revocation.
		live, err := store.LockLiveGuardianAccount(ctx, accountID)
		if err != nil {
			return false, failed("lock school membership", MapError(err))
		}
		if !live {
			return false, nil
		}
		active, _, err := store.LockActiveTenantMappingShared(ctx, accountID, tenantID)
		if err != nil {
			return false, failed("lock school membership", MapError(err))
		}
		return active, nil
	}
}
