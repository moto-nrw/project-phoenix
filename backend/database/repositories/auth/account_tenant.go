package auth

import (
	"context"
	"fmt"

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
// reliably schema-qualify the INSERT INTO clause — it only affects the alias.
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

// ListAccountsByTenantID returns all accounts for a given tenant with their roles and pedagogic info.
// Uses DISTINCT ON to collapse multiple roles per account (picks first alphabetically).
func (r *AccountTenantRepository) ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]auth.TenantAccountInfo, error) {
	var results []auth.TenantAccountInfo
	err := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`DISTINCT ON ("at".account_id) "at".account_id`).
		ColumnExpr(`"a".email`).
		ColumnExpr(`"a".active`).
		ColumnExpr(`COALESCE("p".first_name, '') AS first_name`).
		ColumnExpr(`COALESCE("p".last_name, '') AS last_name`).
		ColumnExpr(`COALESCE("r".name, '') AS role_name`).
		ColumnExpr(`COALESCE("t".role, '') AS pedagogic_role`).
		ColumnExpr(`"at".status`).
		TableExpr(`auth.account_tenants AS "at"`).
		Join(`INNER JOIN auth.accounts AS "a" ON "a".id = "at".account_id`).
		Join(`LEFT JOIN auth.account_roles AS "ar" ON "ar".account_id = "at".account_id AND "ar".tenant_id = ?`, tenantID).
		Join(`LEFT JOIN auth.roles AS "r" ON "r".id = "ar".role_id`).
		Join(`LEFT JOIN users.persons AS "p" ON "p".account_id = "at".account_id AND "p".tenant_id = ?`, tenantID).
		Join(`LEFT JOIN users.staff AS "s" ON "s".person_id = "p".id AND "s".tenant_id = ?`, tenantID).
		Join(`LEFT JOIN users.teachers AS "t" ON "t".staff_id = "s".id AND "t".tenant_id = ?`, tenantID).
		Where(`"at".tenant_id = ?`, tenantID).
		OrderExpr(`"at".account_id, "r".name ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return results, nil
}
