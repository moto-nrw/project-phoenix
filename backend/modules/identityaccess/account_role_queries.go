package identityaccess

import "context"

// AccountRoleQueries supplies role facts without persistence rows. Assigned
// names belong to the school in context; system role IDs are platform-wide.
type AccountRoleQueries interface {
	ListSchoolAccountRoleNames(context.Context, int64) ([]string, error)
	FindSystemRoleID(context.Context, string) (int64, bool, error)
}

func (m *Module) ListSchoolAccountRoleNames(ctx context.Context, accountID int64) ([]string, error) {
	return m.engine.ListSchoolAccountRoleNames(ctx, accountID)
}

func (m *Module) FindSystemRoleID(ctx context.Context, name string) (int64, bool, error) {
	return m.engine.FindSystemRoleID(ctx, name)
}
