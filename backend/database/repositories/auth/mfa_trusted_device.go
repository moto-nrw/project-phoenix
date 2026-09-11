package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	mfaTrustedDeviceTable      = "auth.mfa_trusted_devices"
	mfaTrustedDeviceTableAlias = `auth.mfa_trusted_devices AS "mfa_trusted_device"`
)

// MFATrustedDeviceRepository implements auth.MFATrustedDeviceRepository.
type MFATrustedDeviceRepository struct {
	*base.Repository[*auth.MFATrustedDevice]
	db *bun.DB
}

// NewMFATrustedDeviceRepository creates a new repository for trusted-device records.
func NewMFATrustedDeviceRepository(db *bun.DB) auth.MFATrustedDeviceRepository {
	return &MFATrustedDeviceRepository{
		Repository: base.NewRepository[*auth.MFATrustedDevice](db, mfaTrustedDeviceTable, "MfaTrustedDevice"),
		db:         db,
	}
}

// Create persists a new trusted-device record after light validation.
func (r *MFATrustedDeviceRepository) Create(ctx context.Context, device *auth.MFATrustedDevice) error {
	if device == nil {
		return fmt.Errorf("mfa trusted device cannot be nil")
	}
	if device.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if device.TenantID == 0 {
		return fmt.Errorf("tenant_id is required")
	}
	if device.TokenHash == "" {
		return fmt.Errorf("token_hash is required")
	}
	if device.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return r.Repository.Create(ctx, device)
}

// FindActiveByAccountTenantAndTokenHash returns the device record matching the
// cookie's hashed token iff it is still active (not expired, not revoked) AND
// belongs to the given (account, tenant) pair. The tenant filter is the
// per-tenant trust boundary from #1430 review item #9: a cookie issued for
// tenant A must never resolve to a row from tenant B.
func (r *MFATrustedDeviceRepository) FindActiveByAccountTenantAndTokenHash(ctx context.Context, accountID, tenantID int64, tokenHash string) (*auth.MFATrustedDevice, error) {
	device := new(auth.MFATrustedDevice)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(device).
		ModelTableExpr(mfaTrustedDeviceTableAlias).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("token_hash = ?", tokenHash).
		Where("revoked_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active mfa trusted device", Err: base.TranslateNotFound(err)}
	}
	return device, nil
}

// ListActiveByAccountTenant returns every non-revoked, non-expired device for
// (account, tenant). Security settings are per-tenant so the list is too —
// users see only the devices they trusted while logged into the current
// tenant. (#1430 review item #9)
func (r *MFATrustedDeviceRepository) ListActiveByAccountTenant(ctx context.Context, accountID, tenantID int64) ([]*auth.MFATrustedDevice, error) {
	var devices []*auth.MFATrustedDevice
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(mfaTrustedDeviceTableAlias).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("revoked_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Order("last_used_at DESC NULLS LAST", "created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list active mfa trusted devices", Err: base.TranslateNotFound(err)}
	}
	return devices, nil
}

// UpdateLastUsedAt stamps a device row when a login was waved through with its cookie.
func (r *MFATrustedDeviceRepository) UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error {
	device := &auth.MFATrustedDevice{Model: modelBase.Model{ID: id}, LastUsedAt: &when}
	_, err := r.UpdateColumns(ctx, device, "last_used_at")
	return err
}

// Revoke marks a single device as revoked (user clicked "remove" in settings).
func (r *MFATrustedDeviceRepository) Revoke(ctx context.Context, id int64, revokedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.MFATrustedDevice)(nil)).
		ModelTableExpr(mfaTrustedDeviceTable).
		Set("revoked_at = ?", revokedAt).
		Where(whereID, id).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke mfa trusted device", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(res, 1, "revoke mfa trusted device")
}

// RevokeAllByAccountID revokes every active device for an account.
// Triggered when the user disables MFA or an admin override fires.
func (r *MFATrustedDeviceRepository) RevokeAllByAccountID(ctx context.Context, accountID int64, revokedAt time.Time) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.MFATrustedDevice)(nil)).
		ModelTableExpr(mfaTrustedDeviceTable).
		Set("revoked_at = ?", revokedAt).
		Where("account_id = ?", accountID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke all mfa trusted devices for account", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// RevokeAllByAccountTenant revokes every active device for the
// (account, tenant) pair without touching devices the same account
// trusted in other tenants. Used by tenant-scoped force_off writes —
// flipping the override in tenant A must invalidate the user's tenant
// A trust cookies but must not yank their tenant B cookies.
func (r *MFATrustedDeviceRepository) RevokeAllByAccountTenant(ctx context.Context, accountID, tenantID int64, revokedAt time.Time) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.MFATrustedDevice)(nil)).
		ModelTableExpr(mfaTrustedDeviceTable).
		Set("revoked_at = ?", revokedAt).
		Where("account_id = ?", accountID).
		Where("tenant_id = ?", tenantID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke all mfa trusted devices for account+tenant", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// DeleteExpired prunes devices past their TTL.
func (r *MFATrustedDeviceRepository) DeleteExpired(ctx context.Context) (int, error) {
	deleted, err := r.DeleteBefore(ctx, "expires_at", time.Now(), "delete expired mfa trusted devices")
	return int(deleted), err
}
