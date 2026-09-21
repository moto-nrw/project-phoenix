package users

import (
	"context"
)

// The school mapping, the role assignments and the roles belong to Identity
// & Access (#2721). Repositories here state the owner facts they filter by as
// plain function types, so this package does not depend on that owner; the
// composition root binds them to the owner queries.

// PortalMembershipQuery maps the requested accounts to schools where their
// account, school membership, and guardian role make the portal reachable.
// It does not authorize access to any child.
type PortalMembershipQuery func(context.Context, []int64) (map[int64][]int64, error)

// SchoolRoleClass is the owner's classification of the roles one account
// holds at one school.
type SchoolRoleClass struct {
	AccountID   int64
	IsAdmin     bool
	IsLehrkraft bool
}

// SchoolRoleClassQuery classifies the roles the accounts hold at the tenant.
// Accounts without a role there are absent from the result.
type SchoolRoleClassQuery func(ctx context.Context, tenantID int64, accountIDs []int64) ([]SchoolRoleClass, error)
