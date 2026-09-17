package auth

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
		Where(`"account_tenant".status = ?`, auth.AccountTenantStatusActive)
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
		Where(`LOWER("role".name) = ?`, strings.ToLower(auth.BaseRoleGuardian))
}

// SchoolRoleClass classifies the roles one account holds at one school.
// IsAdmin is the system admin role itself (seeded without a base_role) or a
// custom role whose base_role is admin; IsLehrkraft is the platform system
// role of that name, so a school's own custom role sharing the label does not
// count.
type SchoolRoleClass struct {
	AccountID   int64 `bun:"account_id"`
	IsAdmin     bool  `bun:"is_admin"`
	IsLehrkraft bool  `bun:"is_lehrkraft"`
}

// ClassifySchoolRoles returns one class per account that holds at least one
// role at the tenant. Accounts without a role there are absent.
func (r *AccountRoleRepository) ClassifySchoolRoles(ctx context.Context, tenantID int64, accountIDs []int64) ([]SchoolRoleClass, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	var rows []SchoolRoleClass
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(accountRoleTableAlias).
		ColumnExpr(`"account_role".account_id AS account_id`).
		ColumnExpr(`COALESCE(bool_or("role".base_role = ? OR ("role".is_system AND lower(btrim("role".name)) = ?)), false) AS is_admin`, auth.BaseRoleAdmin, auth.BaseRoleAdmin).
		ColumnExpr(`COALESCE(bool_or("role".is_system AND lower(btrim("role".name)) = 'lehrkraft'), false) AS is_lehrkraft`).
		Join(`JOIN auth.roles AS "role" ON "role".id = "account_role".role_id`).
		Where(`"account_role".tenant_id = ?`, tenantID).
		Where(`"account_role".account_id IN (?)`, bun.List(accountIDs)).
		GroupExpr(`"account_role".account_id`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "classify school roles", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}
