package identityaccess

import "context"

// AccountRoleQueries supplies role facts without persistence rows. Assigned
// names belong to the school in context; system role IDs are platform-wide.
type AccountRoleQueries interface {
	// FindActiveSchoolMemberships returns only active mappings within both
	// supplied sets. It does not infer account activation or portal authority.
	FindActiveSchoolMemberships(context.Context, []int64, []int64) (map[int64][]int64, error)
	// ListActiveAccountSchoolIDs reports active memberships in creation order.
	// Account and school activation are separate facts. Ambient RLS is preserved.
	ListActiveAccountSchoolIDs(context.Context, int64) ([]int64, error)
	ListSchoolAccountRoleNames(context.Context, int64) ([]string, error)
	FindSystemRoleID(context.Context, string) (int64, bool, error)
}

func (m *Module) FindActiveSchoolMemberships(ctx context.Context, accountIDs, schoolIDs []int64) (map[int64][]int64, error) {
	return m.engine.FindActiveSchoolMemberships(ctx, accountIDs, schoolIDs)
}

func (m *Module) ListActiveAccountSchoolIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return m.engine.ListActiveAccountSchoolIDs(ctx, accountID)
}

func (m *Module) ListSchoolAccountRoleNames(ctx context.Context, accountID int64) ([]string, error) {
	return m.engine.ListSchoolAccountRoleNames(ctx, accountID)
}

func (m *Module) FindSystemRoleID(ctx context.Context, name string) (int64, bool, error) {
	return m.engine.FindSystemRoleID(ctx, name)
}
