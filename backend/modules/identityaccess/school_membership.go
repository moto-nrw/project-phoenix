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

// SchoolMembershipLock holds an account's membership of the school in
// context for the caller's write (#6529): it reports whether the mapping is
// active and, if so, pins it FOR SHARE until the caller's school transaction
// ends, so a deactivation or deletion of the mapping waits for that write
// instead of committing under it. It runs only inside the caller's school
// transaction and takes no account or school lock; callers take it before any
// lock of the school's own rows, the order of Konto → Schule → Profil.
type SchoolMembershipLock func(ctx context.Context, accountID int64) (bool, error)
