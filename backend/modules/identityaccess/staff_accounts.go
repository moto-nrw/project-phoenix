package identityaccess

import "context"

// StaffAccountQueries supplies account facts for staff projections, not credentials
// or persistence rows. The caller resolves authorized staff/person links first.
// Explicit tenant IDs select memberships at that school; the ambient transaction
// and its RLS remain in force. Role-name counts preserve the historical listing:
// one result per assignment visible to the caller, without a separate tenant filter.
// Permission names are the union of positive role/direct grants, not an authorization
// decision; direct denials do not override role grants in this staff projection.
// Email lookup is global and restricted to the supplied account IDs.
type StaffAccountQueries interface {
	ClassifySchoolRoles(ctx context.Context, tenantID int64, accountIDs []int64) ([]SchoolRoleClass, error)
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
	CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, error)
	ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// SchoolRoleClass reports role categories at one school. Accounts with no
// assignment there are absent. Admin includes custom admin-tier roles;
// Lehrkraft requires the system role, not merely a matching custom name.
type SchoolRoleClass struct {
	AccountID   int64
	IsAdmin     bool
	IsLehrkraft bool
}

func (m *Module) ClassifySchoolRoles(ctx context.Context, tenantID int64, accountIDs []int64) ([]SchoolRoleClass, error) {
	return m.engine.ClassifySchoolRoles(ctx, tenantID, accountIDs)
}

func (m *Module) ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error) {
	return m.engine.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
}

func (m *Module) FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error) {
	return m.engine.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, accountIDs, tenantID)
}

func (m *Module) CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, error) {
	return m.engine.CountRoleNameMatchesByAccountIDs(ctx, accountIDs, roleNames)
}

func (m *Module) ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error) {
	return m.engine.ListAccountIDsWithSystemRoleNames(ctx, accountIDs, roleNames, tenantID)
}

func (m *Module) ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return m.engine.ListAccountEmails(ctx, accountIDs)
}
