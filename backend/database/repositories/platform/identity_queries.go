package platform

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

// ActiveMembershipQuery returns the Identity & Access owner statement
// selecting (account_id, tenant_id) of every ACTIVE school mapping. The
// school lookups and the operator dashboard counts join it instead of
// reading auth.account_tenants (#2721); it is a plain function type so this
// package does not depend on that owner.
type ActiveMembershipQuery func(ctx context.Context) *bun.SelectQuery

var errActiveMembershipQueryRequired = errors.New("platform repositories: active membership query is not bound")

func activeMemberships(ctx context.Context, query ActiveMembershipQuery) (*bun.SelectQuery, error) {
	if query == nil {
		return nil, errActiveMembershipQueryRequired
	}
	return query(ctx), nil
}
