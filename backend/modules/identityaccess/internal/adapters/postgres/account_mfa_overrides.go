package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

const accountMFAOverrideColumns = "id, account_id, tenant_id, override, set_by, set_by_type, reason"

func (r *AccountMFARecords) FindGlobalOverride(ctx context.Context, accountID int64) (domain.AccountMFAOverride, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountMFAOverride{}, false, err
	}
	var row domain.AccountMFAOverride
	err = db.NewRaw("SELECT "+accountMFAOverrideColumns+" FROM auth.mfa_overrides WHERE account_id = ? AND tenant_id IS NULL LIMIT 1", accountID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountMFAOverride{}, false, nil
	}
	return row, err == nil, err
}

func (r *AccountMFARecords) FindTenantOverride(ctx context.Context, accountID, tenantID int64) (domain.AccountMFAOverride, bool, error) {
	if tenantID == 0 {
		return domain.AccountMFAOverride{}, false, nil
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountMFAOverride{}, false, err
	}
	var row domain.AccountMFAOverride
	err = db.NewRaw("SELECT "+accountMFAOverrideColumns+" FROM auth.mfa_overrides WHERE account_id = ? AND tenant_id = ? LIMIT 1", accountID, tenantID).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountMFAOverride{}, false, nil
	}
	return row, err == nil, err
}

func (r *AccountMFARecords) UpsertGlobalOverride(ctx context.Context, override domain.AccountMFAOverride) error {
	if override.AccountID == 0 || override.TenantID != nil || override.SetByType != "operator" {
		return fmt.Errorf("global mfa override requires an account, no tenant and an operator")
	}
	if override.Override != "force_on" && override.Override != "force_off" {
		return fmt.Errorf("override must be force_off or force_on")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`INSERT INTO auth.mfa_overrides (account_id, tenant_id, override, set_by, set_by_type, reason)
 VALUES (?, NULL, ?, ?, ?, ?)
 ON CONFLICT (account_id) WHERE tenant_id IS NULL DO UPDATE
 SET override = EXCLUDED.override, set_by = EXCLUDED.set_by, set_by_type = EXCLUDED.set_by_type,
 reason = EXCLUDED.reason, updated_at = CURRENT_TIMESTAMP`,
		override.AccountID, override.Override, override.SetBy, override.SetByType, override.Reason).Exec(ctx)
	return err
}

func (r *AccountMFARecords) UpsertTenantOverride(ctx context.Context, override domain.AccountMFAOverride) error {
	if override.AccountID == 0 || override.TenantID == nil || *override.TenantID == 0 {
		return fmt.Errorf("tenant mfa override requires an account and tenant")
	}
	if override.Override != "force_on" && override.Override != "force_off" {
		return fmt.Errorf("override must be force_off or force_on")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`INSERT INTO auth.mfa_overrides (account_id, tenant_id, override, set_by, set_by_type, reason)
 VALUES (?, ?, ?, ?, ?, ?)
 ON CONFLICT (account_id, tenant_id) WHERE tenant_id IS NOT NULL DO UPDATE
 SET override = EXCLUDED.override, set_by = EXCLUDED.set_by, set_by_type = EXCLUDED.set_by_type,
 reason = EXCLUDED.reason, updated_at = CURRENT_TIMESTAMP`,
		override.AccountID, override.TenantID, override.Override, override.SetBy, override.SetByType, override.Reason).Exec(ctx)
	return err
}

func (r *AccountMFARecords) DeleteGlobalOverride(ctx context.Context, accountID int64) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("DELETE FROM auth.mfa_overrides WHERE account_id = ? AND tenant_id IS NULL", accountID).Exec(ctx)
	return err
}

func (r *AccountMFARecords) DeleteTenantOverride(ctx context.Context, accountID, tenantID int64) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant_id is required")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("DELETE FROM auth.mfa_overrides WHERE account_id = ? AND tenant_id = ?", accountID, tenantID).Exec(ctx)
	return err
}
