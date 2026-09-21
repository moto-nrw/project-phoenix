package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

const accountTrustedDeviceColumns = "id, account_id, tenant_id, token_hash, user_agent, ip_address, expires_at, last_used_at, revoked_at, created_at, updated_at"

func (r *AccountMFARecords) CreateTrustedDevice(ctx context.Context, device domain.AccountTrustedDevice) (domain.AccountTrustedDevice, error) {
	if device.AccountID == 0 || device.TenantID == 0 || device.TokenHash == "" || device.ExpiresAt.IsZero() {
		return domain.AccountTrustedDevice{}, fmt.Errorf("trusted device requires account, tenant, hash and expiry")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountTrustedDevice{}, err
	}
	var row domain.AccountTrustedDevice
	err = db.NewRaw(`INSERT INTO auth.mfa_trusted_devices
 (account_id, tenant_id, token_hash, user_agent, ip_address, expires_at, last_used_at, revoked_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+accountTrustedDeviceColumns,
		device.AccountID, device.TenantID, device.TokenHash, device.UserAgent, nullableMFAIPAddress(device.IPAddress), device.ExpiresAt, device.LastUsedAt, device.RevokedAt).Scan(ctx, &row)
	return row, err
}

func (r *AccountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID int64, tokenHash string) (domain.AccountTrustedDevice, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountTrustedDevice{}, false, err
	}
	var row domain.AccountTrustedDevice
	err = db.NewRaw("SELECT "+accountTrustedDeviceColumns+` FROM auth.mfa_trusted_devices
 WHERE account_id = ? AND tenant_id = ? AND token_hash = ? AND revoked_at IS NULL AND expires_at > ? LIMIT 1`,
		accountID, tenantID, tokenHash, time.Now()).Scan(ctx, &row)
	return row, err == nil, err
}

func (r *AccountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID int64) ([]domain.AccountTrustedDevice, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]domain.AccountTrustedDevice, 0)
	err = db.NewRaw("SELECT "+accountTrustedDeviceColumns+` FROM auth.mfa_trusted_devices
 WHERE account_id = ? AND tenant_id = ? AND revoked_at IS NULL AND expires_at > ?
 ORDER BY last_used_at DESC NULLS LAST, created_at DESC`, accountID, tenantID, time.Now()).Scan(ctx, &rows)
	return rows, err
}

func (r *AccountMFARecords) TouchTrustedDevice(ctx context.Context, id int64, usedAt time.Time) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("UPDATE auth.mfa_trusted_devices SET last_used_at = ? WHERE id = ?", usedAt, id).Exec(ctx)
	return err
}

func (r *AccountMFARecords) RevokeTrustedDevice(ctx context.Context, id int64, revokedAt time.Time) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	return requireMFATransition(db.NewRaw("UPDATE auth.mfa_trusted_devices SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL", revokedAt, id).Exec(ctx))
}

func (r *AccountMFARecords) RevokeAllTrustedDevices(ctx context.Context, accountID int64, revokedAt time.Time) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("UPDATE auth.mfa_trusted_devices SET revoked_at = ? WHERE account_id = ? AND revoked_at IS NULL", revokedAt, accountID).Exec(ctx)
	return err
}

func (r *AccountMFARecords) RevokeTenantTrustedDevices(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	db, err := r.store.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw("UPDATE auth.mfa_trusted_devices SET revoked_at = ? WHERE account_id = ? AND tenant_id = ? AND revoked_at IS NULL", revokedAt, accountID, tenantID).Exec(ctx)
	return err
}
