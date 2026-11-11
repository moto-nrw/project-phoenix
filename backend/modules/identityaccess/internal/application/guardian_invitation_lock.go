package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// NewGuardianInvitationAccountLock binds the invitation's first locks without
// composing the full account lifecycle. Locks must survive until its commit.
func NewGuardianInvitationAccountLock(store ports.GuardianInvitationLockStore, tenantOf func(context.Context) int64, transaction func(context.Context) (any, bool)) identityaccess.GuardianInvitationAccountLock {
	return func(ctx context.Context, inviterID int64, email string) error {
		if _, open := transaction(ctx); !open || tenantOf(ctx) <= 0 {
			return failed("lock invitation accounts", errMembershipLockOutsideTransaction)
		}
		return store.LockGuardianInvitationAccounts(ctx, inviterID, strings.ToLower(strings.TrimSpace(email)))
	}
}
