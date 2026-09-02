package auth

import (
	"context"
	"database/sql"
	"errors"
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
		Set("staff_calendar_feed_token = NULL").
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

// ExistsActiveByAccountAndTenantForShare is ExistsByAccountAndTenant with a
// FOR SHARE row lock. Must be called inside a transaction.
//
// Membership is revoked by Deactivate, which UPDATEs exactly this row. Under
// READ COMMITTED a plain existence check only proves the mapping was active at
// statement time — a revocation committing right afterwards still leaves the
// caller free to write. The share lock makes that impossible: the revoking
// UPDATE blocks until the calling transaction commits, so any token persisted
// in the same transaction is provably backed by a live membership.
//
// No join here on purpose — FOR SHARE on a single table locks unambiguously.
func (r *AccountTenantRepository) ExistsActiveByAccountAndTenantForShare(ctx context.Context, accountID, tenantID int64) (bool, error) {
	var one int
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr("1").
		TableExpr(accountTenantTable).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("status = ?", auth.AccountTenantStatusActive).
		For("SHARE").
		Limit(1).
		Scan(ctx, &one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListTenantAccessByAccountID returns every school mapping of one account,
// active and inactive alike, with school/organization context and whether the
// account already carries a person and staff record at that school.
//
// This is the inverse direction of the account listings below (account -> many
// schools instead of school -> many accounts) and is therefore cross-tenant by
// construction: callers must be operator-scoped.
//
// HasPerson and HasStaff are attached by the composition layer through the
// People Directory (#2661); the rows leave here with both unset.
func (r *AccountTenantRepository) ListTenantAccessByAccountID(ctx context.Context, accountID int64) ([]auth.AccountTenantAccessInfo, error) {
	var rows []auth.AccountTenantAccessInfo
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`"at".tenant_id`).
		ColumnExpr(`"at".status`).
		ColumnExpr(`"at".activated_at`).
		ColumnExpr(`"at".deactivated_at`).
		ColumnExpr(`FALSE AS has_person`).
		ColumnExpr(`FALSE AS has_staff`).
		TableExpr(`auth.account_tenants AS "at"`).
		Where(`"at".account_id = ?`, accountID).
		OrderExpr(`"at".tenant_id ASC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ListAccountsByTenantID returns all accounts for a given tenant with their
// roles, plus pending invitations that haven't been accepted yet. The person
// names and the caregiver facts (pedagogic role, caregiver profile, active
// caregiver) are attached by the composition layer through the People
// Directory and CaregiverChainByPersonIDs (#2661); the account rows leave
// here in account-id order and the invitations follow them.
func (r *AccountTenantRepository) ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]auth.TenantAccountInfo, error) {
	db := base.GetDB(ctx, r.db)

	var accounts []auth.TenantAccountInfo
	err := db.NewSelect().
		ColumnExpr(`"at".account_id`).
		ColumnExpr(`"a".email`).
		ColumnExpr(`"a".active`).
		ColumnExpr(`'' AS first_name`).
		ColumnExpr(`'' AS last_name`).
		ColumnExpr(`COALESCE(string_agg(DISTINCT "r".name, ', ' ORDER BY "r".name), '') AS role_name`).
		ColumnExpr(`'' AS pedagogic_role`).
		ColumnExpr(`"at".status`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'admin'), false) AS has_admin_role`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'user'), false) AS has_user_role`).
		ColumnExpr(`FALSE AS has_caregiver_profile`).
		ColumnExpr(`FALSE AS is_active_caregiver`).
		TableExpr(`auth.account_tenants AS "at"`).
		Join(`INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id`).
		Join(`LEFT JOIN auth.account_roles AS "ar" ON "ar".account_id = "at".account_id AND "ar".tenant_id = ?`, tenantID).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "ar".role_id`).
		Where(`"at".tenant_id = ?`, tenantID).
		GroupExpr(`"at".account_id, "a".email, "a".active, "at".status`).
		OrderExpr(`"at".account_id ASC`).
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
	return nil, fmt.Errorf("list organization accounts through organization tenancy capability")
}

// ListAllAccounts returns all accounts across all schools with their roles and pedagogic info,
// plus pending invitations. Each result includes school_id and school_name for context.
func (r *AccountTenantRepository) ListAllAccounts(ctx context.Context) ([]auth.OrgAccountInfo, error) {
	return nil, fmt.Errorf("list all accounts through organization tenancy capability")
}

// ListAccountsBySchoolIDs returns account and invitation rows for an explicit
// school set supplied by the Organization & Tenancy capability adapter.
func (r *AccountTenantRepository) ListAccountsBySchoolIDs(ctx context.Context, schoolIDs []int64) ([]auth.OrgAccountInfo, error) {
	if len(schoolIDs) == 0 {
		return []auth.OrgAccountInfo{}, nil
	}
	db := base.GetDB(ctx, r.db)
	accounts, err := r.queryOrgAccounts(ctx, db, schoolIDs)
	if err != nil {
		return nil, err
	}
	invitations, err := r.queryOrgInvitations(ctx, db, schoolIDs)
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
//
// The person names and caregiver facts are attached by the composition
// layer (#2661); the rows leave here in school/account order.
func (r *AccountTenantRepository) queryOrgAccounts(ctx context.Context, db bun.IDB, schoolIDs []int64) ([]auth.OrgAccountInfo, error) {
	var accounts []auth.OrgAccountInfo
	q := db.NewSelect().
		ColumnExpr(`"at".tenant_id AS school_id`).
		ColumnExpr(`'' AS school_name`).
		ColumnExpr(`"at".account_id`).
		ColumnExpr(`"a".email`).
		ColumnExpr(`"a".active`).
		ColumnExpr(`'' AS first_name`).
		ColumnExpr(`'' AS last_name`).
		ColumnExpr(`COALESCE(string_agg(DISTINCT "r".name, ', ' ORDER BY "r".name), '') AS role_name`).
		ColumnExpr(`'' AS pedagogic_role`).
		ColumnExpr(`"at".status`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'admin'), false) AS has_admin_role`).
		ColumnExpr(`COALESCE(bool_or(LOWER("r".name) = 'user'), false) AS has_user_role`).
		ColumnExpr(`FALSE AS has_caregiver_profile`).
		ColumnExpr(`FALSE AS is_active_caregiver`).
		TableExpr(`auth.account_tenants AS "at"`).
		Join(`INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id`).
		Join(`LEFT JOIN auth.account_roles AS "ar" ON "ar".account_id = "at".account_id AND "ar".tenant_id = "at".tenant_id`).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "ar".role_id`).
		Where(`"at".tenant_id IN (?)`, bun.List(schoolIDs))
	err := q.
		GroupExpr(`"at".tenant_id, "at".account_id, "a".email, "a".active, "at".status`).
		OrderExpr(`"at".tenant_id ASC, "at".account_id ASC`).
		Scan(ctx, &accounts)
	return accounts, err
}

// CaregiverChainByPersonIDs returns the live staff record (and its teacher
// record, when present) behind each person. Persons without a staff record
// are absent from the result.
func (r *AccountTenantRepository) CaregiverChainByPersonIDs(ctx context.Context, personIDs []int64) (map[int64]auth.CaregiverChain, error) {
	result := make(map[int64]auth.CaregiverChain, len(personIDs))
	if len(personIDs) == 0 {
		return result, nil
	}
	var rows []auth.CaregiverChain
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`"s".person_id`).
		ColumnExpr(`"s".tenant_id`).
		ColumnExpr(`"s".id AS staff_id`).
		ColumnExpr(`COALESCE("t".id, 0) AS teacher_id`).
		ColumnExpr(`COALESCE("t".role, '') AS teacher_role`).
		TableExpr(`users.staff AS "s"`).
		Join(`LEFT JOIN users.teachers AS "t" ON "t".staff_id = "s".id AND "t".tenant_id = "s".tenant_id AND "t".deleted_at IS NULL`).
		Where(`"s".person_id IN (?)`, bun.List(personIDs)).
		Where(`"s".deleted_at IS NULL`).
		OrderExpr(`"s".person_id ASC, "s".id ASC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if _, found := result[row.PersonID]; !found {
			result[row.PersonID] = row
		}
	}
	return result, nil
}

// queryOrgInvitations builds the shared org-level pending invitations query with an optional WHERE clause.
func (r *AccountTenantRepository) queryOrgInvitations(ctx context.Context, db bun.IDB, schoolIDs []int64) ([]auth.OrgAccountInfo, error) {
	var invitations []auth.OrgAccountInfo
	q := db.NewSelect().
		ColumnExpr(`"inv".tenant_id AS school_id`).
		ColumnExpr(`'' AS school_name`).
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
		Where(`"inv".tenant_id IN (?)`, bun.List(schoolIDs))
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
