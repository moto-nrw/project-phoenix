package authpostgres

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	"github.com/uptrace/bun"
)

// Identity & Access owns the school mappings (auth.account_tenants), the role
// assignments (auth.account_roles) and the roles themselves (auth.roles).
// Repositories of other owners that must filter by those facts join the
// owner queries below instead of naming the tables (#2721). The statements
// run under the caller's transaction. auth.account_roles and auth.roles are
// protected by row-level security, so a school transaction sees only its own
// assignments; auth.account_tenants is not.

// ActiveMemberships is the owner query "every ACTIVE school mapping". It
// selects (account_id, tenant_id) and is meant to be joined or used in a
// row-value IN predicate so the consumer's statement stays one round trip.
// auth.account_tenants carries no row-level security and cross-school
// callers (parent portal, operator dashboard, login) need every school, so
// the consumer must always match tenant_id against its own tenant column.
func (r *AccountTenantRepository) ActiveMemberships(ctx context.Context) *bun.SelectQuery {
	return base.GetDB(ctx, r.db).NewSelect().
		TableExpr(accountTenantTableAlias).
		ColumnExpr(`"account_tenant".account_id`).
		ColumnExpr(`"account_tenant".tenant_id`).
		Where(`"account_tenant".status = ?`, authmodels.AccountTenantStatusActive)
}

// GuardianRoleHolders is the owner query "every account holding the guardian
// base role at a school". It selects (account_id, tenant_id); the role name
// matches case-insensitively like the parent login guardian-role check.
func (r *AccountRoleRepository) GuardianRoleHolders(ctx context.Context) *bun.SelectQuery {
	return base.GetDB(ctx, r.db).NewSelect().
		TableExpr(accountRoleTableAlias).
		ColumnExpr(`"account_role".account_id`).
		ColumnExpr(`"account_role".tenant_id`).
		Join(`INNER JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`LOWER("role".name) = ?`, strings.ToLower(authmodels.BaseRoleGuardian))
}
