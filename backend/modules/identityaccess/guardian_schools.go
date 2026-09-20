package identityaccess

import "context"

// GuardianSchools lists active school memberships carrying the guardian role.
// The caller supplies the authenticated account and an administrative transaction
// when querying across schools. Results retain membership creation order.
type GuardianSchools interface {
	ListGuardianSchoolIDs(context.Context, int64) ([]int64, error)
}

func (m *Module) ListGuardianSchoolIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return m.engine.ListGuardianSchoolIDs(ctx, accountID)
}
