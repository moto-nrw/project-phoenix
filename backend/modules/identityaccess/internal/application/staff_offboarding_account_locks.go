package application

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// StaffOffboardingAccountLocks is the account-only step of a planned exit.
// Access snapshots stay with the preview and execution of each offboarding.
type StaffOffboardingAccountLocks struct {
	Accounts ports.StaffOffboardingAccountLocks
}

func (l StaffOffboardingAccountLocks) LockStaffOffboardingAccounts(ctx context.Context, accountIDs []int64) error {
	_, inTransaction := tenant.TransactionFromContext(ctx)
	if tenant.FromContext(ctx) <= 0 || !inTransaction {
		return errors.New("tenant transaction is required for staff offboarding account locks")
	}
	ids := slices.Clone(accountIDs)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	for _, id := range ids {
		if id <= 0 {
			return errors.New("account is required for staff offboarding")
		}
		if err := tenant.OrderLock(ctx, tenant.LockAccount, tenant.LockIDKey(id)); err != nil {
			return err
		}
	}
	if len(ids) == 0 {
		return nil
	}
	locked, _, err := l.Accounts.LockLoginAccounts(ctx, ids)
	if err != nil {
		return err
	}
	if len(locked) != len(ids) {
		return identityaccess.ErrAccountNotFound
	}
	return nil
}
