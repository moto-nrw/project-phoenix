package identityaccess

import "context"

// StaffOffboardingAccountLocks takes the accounts of a planned-exit run in
// ascending order before its school gates, in the caller's transaction.
type StaffOffboardingAccountLocks interface {
	LockStaffOffboardingAccounts(context.Context, []int64) error
}
