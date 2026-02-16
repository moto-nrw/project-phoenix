package auth

import (
	"context"

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
