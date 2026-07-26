package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/uptrace/bun"
)

const (
	accountTenantTable      = "auth.account_tenants"
	accountTenantTableAlias = `auth.account_tenants AS "account_tenant"`
)

// AccountTenantRepository provides access to account-tenant mappings.
type AccountTenantRepository struct {
	db *bun.DB
}

// NewAccountTenantRepository creates a new account-tenant repository.
func NewAccountTenantRepository(db *bun.DB) auth.AccountTenantRepository {
	return &AccountTenantRepository{db: db}
}

// Create inserts a new account-tenant mapping, ignoring duplicates.
// ModelTableExpr is set explicitly because BUN's BeforeAppendModel hook does not
// reliably schema-qualify the INSERT INTO clause, it only affects the alias.
func (r *AccountTenantRepository) Create(ctx context.Context, mapping *auth.AccountTenant) error {
	if mapping == nil {
		return fmt.Errorf("account tenant cannot be nil")
	}
	if mapping.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(mapping).
		ModelTableExpr(accountTenantTable).
		On("CONFLICT (account_id, tenant_id) DO NOTHING").
		Exec(ctx)
	return err
}

// EnsureActive creates or reactivates an account-tenant mapping.
func (r *AccountTenantRepository) EnsureActive(ctx context.Context, mapping *auth.AccountTenant) error {
	if mapping == nil {
		return fmt.Errorf("account tenant cannot be nil")
	}
	if mapping.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if mapping.TenantID == 0 {
		return fmt.Errorf("tenant_id is required")
	}
	mapping.Status = auth.AccountTenantStatusActive
	if mapping.ActivatedAt == nil {
		now := time.Now()
		mapping.ActivatedAt = &now
	}

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(mapping).
		ModelTableExpr(accountTenantTable).
		On(`CONFLICT (account_id, tenant_id) DO UPDATE SET
			status = EXCLUDED.status,
			activated_at = EXCLUDED.activated_at,
			deactivated_at = NULL,
			updated_at = NOW()`).
		Exec(ctx)
	return err
}

// Deactivate marks the mapping for the given account and tenant as inactive.
// The row is kept (with deactivated_at) so EnsureActive can reactivate it on a
// later re-invitation.
func (r *AccountTenantRepository) Deactivate(ctx context.Context, accountID, tenantID int64) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.AccountTenant)(nil)).
		ModelTableExpr(accountTenantTable).
		Set("status = ?", auth.AccountTenantStatusInactive).
		Set("deactivated_at = NOW()").
		Set("updated_at = NOW()").
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Exec(ctx)
	return err
}

// FindActiveByAccountID returns all active tenant mappings for an account.
func (r *AccountTenantRepository) FindActiveByAccountID(ctx context.Context, accountID int64) ([]auth.AccountTenant, error) {
	var items []auth.AccountTenant
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&items).
		ModelTableExpr(accountTenantTableAlias).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".status = ?`, auth.AccountTenantStatusActive).
		OrderExpr(`"account_tenant".created_at ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// FindActiveGuardianByAccountID returns active tenant mappings where the
// account currently has the guardian role.
func (r *AccountTenantRepository) FindActiveGuardianByAccountID(ctx context.Context, accountID int64) ([]auth.AccountTenant, error) {
	var items []auth.AccountTenant
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&items).
		ModelTableExpr(accountTenantTableAlias).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".status = ?`, auth.AccountTenantStatusActive).
		Where(`EXISTS (
			SELECT 1
			FROM auth.account_roles AS "account_role"
			INNER JOIN auth.roles AS "role" ON "role".id = "account_role".role_id
			WHERE "account_role".account_id = "account_tenant".account_id
				AND "account_role".tenant_id = "account_tenant".tenant_id
				AND LOWER("role".name) = ?
		)`, auth.BaseRoleGuardian).
		OrderExpr(`"account_tenant".created_at ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ExistsByAccountAndTenant checks if an active mapping exists for the given account and tenant.
func (r *AccountTenantRepository) ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	exists, err := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(accountTenantTable).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("status = ?", auth.AccountTenantStatusActive).
		Exists(ctx)
	return exists, err
}

// ListAccountsByTenantID returns all accounts for a given tenant with their roles and pedagogic info,
// plus pending invitations that haven't been accepted yet.
func (r *AccountTenantRepository) ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]auth.TenantAccountInfo, error) {
	db := base.GetDB(ctx, r.db)

	var accounts []auth.TenantAccountInfo
	err := db.NewSelect().
		ColumnExpr(`"at".account_id`).
		ColumnExpr(`"a".email`).
		ColumnExpr(`"a".active`).
		ColumnExpr(`COALESCE("p".first_name, '') AS first_name`).
		ColumnExpr(`COALESCE("p".last_name, '') AS last_name`).
		ColumnExpr(`COALESCE(string_agg(DISTINCT "r".name, ', ' ORDER BY "r".name), '') AS role_name`).
		ColumnExpr(`COALESCE("t".role, '') AS pedagogic_role`).
		ColumnExpr(`"at".status`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'admin'), false) AS has_admin_role`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'user'), false) AS has_user_role`).
		ColumnExpr(`("p".id IS NOT NULL AND "s".id IS NOT NULL AND "t".id IS NOT NULL) AS has_caregiver_profile`).
		ColumnExpr(`(COALESCE(bool_or(LOWER("r".name) = 'user'), false) AND "p".id IS NOT NULL AND "s".id IS NOT NULL AND "t".id IS NOT NULL) AS is_active_caregiver`).
		TableExpr(`auth.account_tenants AS "at"`).
		Join(`INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id`).
		Join(`LEFT JOIN auth.account_roles AS "ar" ON "ar".account_id = "at".account_id AND "ar".tenant_id = ?`, tenantID).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "ar".role_id`).
		Join(`LEFT JOIN users.persons AS "p" ON "p".account_id = "at".account_id AND "p".tenant_id = ? AND "p".deleted_at IS NULL`, tenantID).
		Join(`LEFT JOIN users.staff AS "s" ON "s".person_id = "p".id AND "s".tenant_id = ? AND "s".deleted_at IS NULL`, tenantID).
		Join(`LEFT JOIN users.teachers AS "t" ON "t".staff_id = "s".id AND "t".tenant_id = ? AND "t".deleted_at IS NULL`, tenantID).
		Where(`"at".tenant_id = ?`, tenantID).
		GroupExpr(`"at".account_id, "a".email, "a".active, "p".id, "p".first_name, "p".last_name, "s".id, "t".id, "t".role, "at".status`).
		OrderExpr(`"p".last_name ASC, "p".first_name ASC`).
		Scan(ctx, &accounts)
	if err != nil {
		return nil, err
	}

	var invitations []auth.TenantAccountInfo
	err = r.scanPendingInvitationsTenant(ctx, db, tenantID, &invitations)
	if err != nil {
		return nil, err
	}

	return append(accounts, invitations...), nil
}

// scanPendingInvitationsTenant fetches pending invitations for a single tenant.
func (r *AccountTenantRepository) scanPendingInvitationsTenant(ctx context.Context, db bun.IDB, tenantID int64, out *[]auth.TenantAccountInfo) error {
	return db.NewSelect().
		ColumnExpr(`0 AS account_id`).
		ColumnExpr(`"inv".email`).
		ColumnExpr(`false AS active`).
		ColumnExpr(`COALESCE("inv".first_name, '') AS first_name`).
		ColumnExpr(`COALESCE("inv".last_name, '') AS last_name`).
		ColumnExpr(`COALESCE("r".name, '') AS role_name`).
		ColumnExpr(`'' AS pedagogic_role`).
		ColumnExpr(`'invited' AS status`).
		ColumnExpr(`false AS has_admin_role`).
		ColumnExpr(`false AS has_user_role`).
		ColumnExpr(`false AS has_caregiver_profile`).
		ColumnExpr(`false AS is_active_caregiver`).
		TableExpr(`auth.invitation_tokens AS "inv"`).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "inv".role_id`).
		Where(`"inv".tenant_id = ?`, tenantID).
		Where(`"inv".used_at IS NULL`).
		Where(`"inv".expires_at > NOW()`).
		Where(`NOT EXISTS (
			SELECT 1 FROM auth.account_tenants AS "existing"
			INNER JOIN auth.accounts AS "ea" ON "ea".id = "existing".account_id
			WHERE "existing".tenant_id = ? AND "existing".status = ? AND LOWER("ea".email) = LOWER("inv".email)
		)`, tenantID, auth.AccountTenantStatusActive).
		Scan(ctx, out)
}

// ListAccountsByOrganizationID returns all accounts across all schools belonging to an organization,
// plus pending invitations. Each result includes school_id and school_name for context.
func (r *AccountTenantRepository) ListAccountsByOrganizationID(ctx context.Context, organizationID int64) ([]auth.OrgAccountInfo, error) {
	db := base.GetDB(ctx, r.db)

	accounts, err := r.queryOrgAccounts(ctx, db, `"sch".organization_id = ?`, organizationID)
	if err != nil {
		return nil, err
	}

	invitations, err := r.queryOrgInvitations(ctx, db, `"sch".organization_id = ?`, organizationID)
	if err != nil {
		return nil, err
	}

	return append(accounts, invitations...), nil
}

// ListAllAccounts returns all accounts across all schools with their roles and pedagogic info,
// plus pending invitations. Each result includes school_id and school_name for context.
func (r *AccountTenantRepository) ListAllAccounts(ctx context.Context) ([]auth.OrgAccountInfo, error) {
	db := base.GetDB(ctx, r.db)

	accounts, err := r.queryOrgAccounts(ctx, db, "", nil)
	if err != nil {
		return nil, err
	}

	invitations, err := r.queryOrgInvitations(ctx, db, "", nil)
	if err != nil {
		return nil, err
	}

	return append(accounts, invitations...), nil
}

// queryOrgAccounts builds the shared org-level accounts query with an optional WHERE clause.
//
// Accounts whose tenant school is soft-deleted are filtered out unconditionally.
// Schools are only soft-deletable when their organization is still active, and
// an organization can only be soft-deleted once all its schools are in the
// Papierkorb, so filtering on school.deleted_at also covers the org-deleted case.
func (r *AccountTenantRepository) queryOrgAccounts(ctx context.Context, db bun.IDB, whereClause string, arg interface{}) ([]auth.OrgAccountInfo, error) {
	var accounts []auth.OrgAccountInfo
	q := db.NewSelect().
		ColumnExpr(`"at".tenant_id AS school_id`).
		ColumnExpr(`"sch".name AS school_name`).
		ColumnExpr(`"at".account_id`).
		ColumnExpr(`"a".email`).
		ColumnExpr(`"a".active`).
		ColumnExpr(`COALESCE("p".first_name, '') AS first_name`).
		ColumnExpr(`COALESCE("p".last_name, '') AS last_name`).
		ColumnExpr(`COALESCE(string_agg(DISTINCT "r".name, ', ' ORDER BY "r".name), '') AS role_name`).
		ColumnExpr(`COALESCE("t".role, '') AS pedagogic_role`).
		ColumnExpr(`"at".status`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'admin'), false) AS has_admin_role`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'user'), false) AS has_user_role`).
		ColumnExpr(`("p".id IS NOT NULL AND "s".id IS NOT NULL AND "t".id IS NOT NULL) AS has_caregiver_profile`).
		ColumnExpr(`(COALESCE(bool_or(LOWER("r".name) = 'user'), false) AND "p".id IS NOT NULL AND "s".id IS NOT NULL AND "t".id IS NOT NULL) AS is_active_caregiver`).
		TableExpr(`auth.account_tenants AS "at"`).
		Join(`INNER JOIN platform.schools AS "sch" ON "sch".id = "at".tenant_id`).
		Join(`INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id`).
		Join(`LEFT JOIN auth.account_roles AS "ar" ON "ar".account_id = "at".account_id AND "ar".tenant_id = "at".tenant_id`).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "ar".role_id`).
		Join(`LEFT JOIN users.persons AS "p" ON "p".account_id = "at".account_id AND "p".tenant_id = "at".tenant_id AND "p".deleted_at IS NULL`).
		Join(`LEFT JOIN users.staff AS "s" ON "s".person_id = "p".id AND "s".tenant_id = "at".tenant_id AND "s".deleted_at IS NULL`).
		Join(`LEFT JOIN users.teachers AS "t" ON "t".staff_id = "s".id AND "t".tenant_id = "at".tenant_id AND "t".deleted_at IS NULL`).
		Where(`"sch".deleted_at IS NULL`)
	if whereClause != "" {
		q = q.Where(whereClause, arg)
	}
	err := q.
		GroupExpr(`"at".tenant_id, "sch".name, "at".account_id, "a".email, "a".active, "p".id, "p".first_name, "p".last_name, "s".id, "t".id, "t".role, "at".status`).
		OrderExpr(`"sch".name ASC, "p".last_name ASC, "p".first_name ASC`).
		Scan(ctx, &accounts)
	return accounts, err
}

// queryOrgInvitations builds the shared org-level pending invitations query with an optional WHERE clause.
func (r *AccountTenantRepository) queryOrgInvitations(ctx context.Context, db bun.IDB, whereClause string, arg interface{}) ([]auth.OrgAccountInfo, error) {
	var invitations []auth.OrgAccountInfo
	q := db.NewSelect().
		ColumnExpr(`"inv".tenant_id AS school_id`).
		ColumnExpr(`"sch".name AS school_name`).
		ColumnExpr(`0 AS account_id`).
		ColumnExpr(`"inv".email`).
		ColumnExpr(`false AS active`).
		ColumnExpr(`COALESCE("inv".first_name, '') AS first_name`).
		ColumnExpr(`COALESCE("inv".last_name, '') AS last_name`).
		ColumnExpr(`COALESCE("r".name, '') AS role_name`).
		ColumnExpr(`'' AS pedagogic_role`).
		ColumnExpr(`'invited' AS status`).
		ColumnExpr(`false AS has_admin_role`).
		ColumnExpr(`false AS has_user_role`).
		ColumnExpr(`false AS has_caregiver_profile`).
		ColumnExpr(`false AS is_active_caregiver`).
		TableExpr(`auth.invitation_tokens AS "inv"`).
		Join(`INNER JOIN platform.schools AS "sch" ON "sch".id = "inv".tenant_id`).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "inv".role_id`).
		Where(`"sch".deleted_at IS NULL`)
	if whereClause != "" {
		q = q.Where(whereClause, arg)
	}
	err := q.
		Where(`"inv".used_at IS NULL`).
		Where(`"inv".expires_at > NOW()`).
		Where(`NOT EXISTS (
			SELECT 1 FROM auth.account_tenants AS "existing"
			INNER JOIN auth.accounts AS "ea" ON "ea".id = "existing".account_id
			WHERE "existing".tenant_id = "inv".tenant_id AND "existing".status = ? AND LOWER("ea".email) = LOWER("inv".email)
		)`, auth.AccountTenantStatusActive).
		Scan(ctx, &invitations)
	return invitations, err
}
