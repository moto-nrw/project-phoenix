package authpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
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
func NewAccountTenantRepository(db *bun.DB) authmodels.AccountTenantRepository {
	return &AccountTenantRepository{db: db}
}

// Create inserts a new account-tenant mapping, ignoring duplicates.
// ModelTableExpr is set explicitly because BUN's BeforeAppendModel hook does not
// reliably schema-qualify the INSERT INTO clause, it only affects the alias.
func (r *AccountTenantRepository) Create(ctx context.Context, mapping *authmodels.AccountTenant) error {
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
func (r *AccountTenantRepository) EnsureActive(ctx context.Context, mapping *authmodels.AccountTenant) error {
	if mapping == nil {
		return fmt.Errorf("account tenant cannot be nil")
	}
	if mapping.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if mapping.TenantID == 0 {
		return fmt.Errorf("tenant_id is required")
	}
	mapping.Status = authmodels.AccountTenantStatusActive
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

// FindActiveByAccountID returns all active tenant mappings for an account.
func (r *AccountTenantRepository) FindActiveByAccountID(ctx context.Context, accountID int64) ([]authmodels.AccountTenant, error) {
	var items []authmodels.AccountTenant
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&items).
		ModelTableExpr(accountTenantTableAlias).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".status = ?`, authmodels.AccountTenantStatusActive).
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
		Where("status = ?", authmodels.AccountTenantStatusActive).
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
		Where("status = ?", authmodels.AccountTenantStatusActive).
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
