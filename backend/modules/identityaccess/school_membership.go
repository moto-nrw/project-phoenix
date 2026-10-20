package identityaccess

import (
	"context"
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
