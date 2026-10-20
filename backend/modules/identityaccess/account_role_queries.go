package identityaccess

import "context"

// AccountRoleQueries supplies role facts without persistence rows. Assigned
// names belong to the school in context; system role IDs are platform-wide.
type AccountRoleQueries interface {
	// CountActiveAccountsBySchoolGroups counts distinct accounts with active
	// mappings within each supplied school group. Group keys are caller-owned;
	// empty groups count zero. Account/school activation is not inferred, and
	// the caller's ambient transaction and RLS remain in force.
	CountActiveAccountsBySchoolGroups(context.Context, map[int64][]int64) (map[int64]int, error)
	// ListEffectiveAdminAccountIDs lists active accounts with active mappings
	// and admin authority in the context's school. Without a school filter,
	// it retains the caller's ambient database scope; it never elevates access.
	ListEffectiveAdminAccountIDs(context.Context) ([]int64, error)
	// FindActiveSchoolMemberships returns only active mappings within both
	// supplied sets. It does not infer account activation or portal authority.
	FindActiveSchoolMemberships(context.Context, []int64, []int64) (map[int64][]int64, error)
	// ListActiveAccountSchoolIDs reports active memberships in creation order.
	// Account and school activation are separate facts. Ambient RLS is preserved.
	ListActiveAccountSchoolIDs(context.Context, int64) ([]int64, error)
	ListSchoolAccountRoleNames(context.Context, int64) ([]string, error)
	FindSystemRoleID(context.Context, string) (int64, bool, error)
}
