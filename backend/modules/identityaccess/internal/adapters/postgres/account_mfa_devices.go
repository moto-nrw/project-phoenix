package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

const accountTrustedDeviceColumns = "id, account_id, tenant_id, organization_id, token_hash, user_agent, ip_address, expires_at, last_used_at, revoked_at, created_at, updated_at"

// deviceBinding binds the school or the Träger of a trusted device. The
// caterer portal remembers its devices without either (#4497), the
// Träger-Portal with its Träger and no school (#6955): a zero ID is NULL, and
// every lookup compares both with NULL read as zero, so a device of one
// portal never answers for another, nor a Träger's device for another Träger.
func deviceBinding(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

// trustedDevicePortal matches the portal of a device exactly; it takes the
// school and the Träger as its two arguments.
const trustedDevicePortal = `COALESCE(tenant_id, 0) = ? AND COALESCE(organization_id, 0) = ?`

// trustedDeviceCurrentCredential admits a remembered device only while the
// account's credential generation is the one it was remembered under: every
// credential write ends it at every school (#6289).
const trustedDeviceCurrentCredential = `credential_generation = (SELECT credential_account.mfa_reset_generation FROM auth.accounts AS credential_account WHERE credential_account.id = account_id)`

func (r *AccountMFARecords) CreateTrustedDevice(ctx context.Context, device domain.AccountTrustedDevice) (domain.AccountTrustedDevice, error) {
	if device.AccountID == 0 || device.TokenHash == "" || device.ExpiresAt.IsZero() {
		return domain.AccountTrustedDevice{}, fmt.Errorf("trusted device requires account, hash and expiry")
	}
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountTrustedDevice{}, err
	}
	var row domain.AccountTrustedDevice
	err = db.NewRaw(`INSERT INTO auth.mfa_trusted_devices
 (account_id, tenant_id, organization_id, token_hash, user_agent, ip_address, expires_at, last_used_at, revoked_at, credential_generation)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, (SELECT mfa_reset_generation FROM auth.accounts WHERE id = ?)) RETURNING `+accountTrustedDeviceColumns,
		device.AccountID, deviceBinding(device.TenantID), deviceBinding(device.OrganizationID), device.TokenHash, device.UserAgent, nullableMFAIPAddress(device.IPAddress), device.ExpiresAt, device.LastUsedAt, device.RevokedAt, device.AccountID).Scan(ctx, &row)
	return row, err
}

func (r *AccountMFARecords) FindActiveTrustedDevice(ctx context.Context, accountID, tenantID, organizationID int64, tokenHash string) (domain.AccountTrustedDevice, bool, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return domain.AccountTrustedDevice{}, false, err
	}
	var row domain.AccountTrustedDevice
	err = db.NewRaw("SELECT "+accountTrustedDeviceColumns+` FROM auth.mfa_trusted_devices
 WHERE account_id = ? AND `+trustedDevicePortal+` AND token_hash = ? AND revoked_at IS NULL AND expires_at > ?
 AND `+trustedDeviceCurrentCredential+` LIMIT 1`,
		accountID, tenantID, organizationID, tokenHash, time.Now()).Scan(ctx, &row)
	return row, err == nil, err
}

func (r *AccountMFARecords) ListActiveTrustedDevices(ctx context.Context, accountID, tenantID, organizationID int64) ([]domain.AccountTrustedDevice, error) {
	db, err := r.store.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]domain.AccountTrustedDevice, 0)
	err = db.NewRaw("SELECT "+accountTrustedDeviceColumns+` FROM auth.mfa_trusted_devices
 WHERE account_id = ? AND `+trustedDevicePortal+` AND revoked_at IS NULL AND expires_at > ?
 AND `+trustedDeviceCurrentCredential+`
 ORDER BY last_used_at DESC NULLS LAST, created_at DESC`, accountID, tenantID, organizationID, time.Now()).Scan(ctx, &rows)
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
	_, err = db.NewRaw("UPDATE auth.mfa_trusted_devices SET revoked_at = ? WHERE account_id = ? AND COALESCE(tenant_id, 0) = ? AND revoked_at IS NULL", revokedAt, accountID, tenantID).Exec(ctx)
	return err
}
