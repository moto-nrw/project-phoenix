package identityaccess

import "context"

// GuardianPortalQuery reports active portal school memberships for the
// supplied accounts: active account, active mapping, and a guardian role
// assignment at that same school. Ambient transaction/RLS still applies.
// These are reachability facts, not authorization for any child's data.
type GuardianPortalQuery interface {
	FindActiveGuardianMemberships(context.Context, []int64) (map[int64][]int64, error)
}

// GuardianPortalLoginQuery reports the subset of portal memberships whose
// accounts can also authenticate with a password.
type GuardianPortalLoginQuery interface {
	FindLoginReadyGuardianMemberships(context.Context, []int64) (map[int64][]int64, error)
}

func (m *Module) FindActiveGuardianMemberships(ctx context.Context, accountIDs []int64) (map[int64][]int64, error) {
	return m.engine.FindActiveGuardianMemberships(ctx, accountIDs)
}

func (m *Module) FindLoginReadyGuardianMemberships(ctx context.Context, accountIDs []int64) (map[int64][]int64, error) {
	return m.engine.FindLoginReadyGuardianMemberships(ctx, accountIDs)
}
