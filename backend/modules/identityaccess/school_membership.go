package identityaccess

import (
	"context"
	"fmt"
)

// SchoolMembershipQuery answers the membership facts a tenant-scoped owner
// needs to apply school-scoped grants (#2707): whether an account is still an
// active member, which roles it holds at this school, and which accounts are
// members at all. The tenant is always the one in context.
type SchoolMembershipQuery interface {
	HasActiveSchoolMembership(ctx context.Context, accountID int64) (bool, error)
	ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error)
	ListActiveSchoolAccountIDs(ctx context.Context) ([]int64, error)
}

func (m *Module) HasActiveSchoolMembership(ctx context.Context, accountID int64) (bool, error) {
	active, err := m.engine.HasActiveSchoolMembership(ctx, accountID)
	if err != nil {
		return false, fmt.Errorf("identity access: check school membership: %w", err)
	}
	return active, nil
}

func (m *Module) ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error) {
	ids, err := m.engine.ListAccountRoleIDs(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity access: list account roles: %w", err)
	}
	return ids, nil
}

func (m *Module) ListActiveSchoolAccountIDs(ctx context.Context) ([]int64, error) {
	ids, err := m.engine.ListActiveSchoolAccountIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access: list active school accounts: %w", err)
	}
	return ids, nil
}
