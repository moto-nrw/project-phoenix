package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/uptrace/bun"
)

const (
	operatorMFATrustedDeviceTable      = "platform.operator_mfa_trusted_devices"
	operatorMFATrustedDeviceTableAlias = `platform.operator_mfa_trusted_devices AS "operator_mfa_trusted_device"`
)

type OperatorMFATrustedDeviceRepository struct {
	*base.Repository[*platform.OperatorMFATrustedDevice]
	db *bun.DB
}

func NewOperatorMFATrustedDeviceRepository(db *bun.DB) platform.OperatorMFATrustedDeviceRepository {
	return &OperatorMFATrustedDeviceRepository{
		Repository: base.NewRepository[*platform.OperatorMFATrustedDevice](db, operatorMFATrustedDeviceTable, "OperatorMFATrustedDevice"),
		db:         db,
	}
}

func (r *OperatorMFATrustedDeviceRepository) Create(ctx context.Context, device *platform.OperatorMFATrustedDevice) error {
	if device == nil {
		return fmt.Errorf("operator mfa trusted device cannot be nil")
	}
	if device.OperatorID == 0 {
		return fmt.Errorf("operator_id is required")
	}
	if device.TokenHash == "" {
		return fmt.Errorf("token_hash is required")
	}
	if device.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return r.Repository.Create(ctx, device)
}

func (r *OperatorMFATrustedDeviceRepository) FindActiveByOperatorIDAndTokenHash(ctx context.Context, operatorID int64, tokenHash string) (*platform.OperatorMFATrustedDevice, error) {
	device := new(platform.OperatorMFATrustedDevice)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(device).
		ModelTableExpr(operatorMFATrustedDeviceTableAlias).
		Where("operator_id = ?", operatorID).
		Where("token_hash = ?", tokenHash).
		Where("revoked_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active operator mfa trusted device", Err: err}
	}
	return device, nil
}

func (r *OperatorMFATrustedDeviceRepository) ListActiveByOperatorID(ctx context.Context, operatorID int64) ([]*platform.OperatorMFATrustedDevice, error) {
	var devices []*platform.OperatorMFATrustedDevice
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&devices).
		ModelTableExpr(operatorMFATrustedDeviceTableAlias).
		Where("operator_id = ?", operatorID).
		Where("revoked_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Order("last_used_at DESC NULLS LAST", "created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list active operator mfa trusted devices", Err: err}
	}
	return devices, nil
}

func (r *OperatorMFATrustedDeviceRepository) UpdateLastUsedAt(ctx context.Context, id int64, when time.Time) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorMFATrustedDevice)(nil)).
		ModelTableExpr(operatorMFATrustedDeviceTable).
		Set("last_used_at = ?", when).
		Where(platformWhereID, id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update operator mfa trusted device last_used_at", Err: err}
	}
	return nil
}

func (r *OperatorMFATrustedDeviceRepository) Revoke(ctx context.Context, id int64, revokedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorMFATrustedDevice)(nil)).
		ModelTableExpr(operatorMFATrustedDeviceTable).
		Set("revoked_at = ?", revokedAt).
		Where(platformWhereID, id).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke operator mfa trusted device", Err: err}
	}
	return base.AssertRowsAffected(res, 1, "revoke operator mfa trusted device")
}

func (r *OperatorMFATrustedDeviceRepository) RevokeAllByOperatorID(ctx context.Context, operatorID int64, revokedAt time.Time) error {
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*platform.OperatorMFATrustedDevice)(nil)).
		ModelTableExpr(operatorMFATrustedDeviceTable).
		Set("revoked_at = ?", revokedAt).
		Where("operator_id = ?", operatorID).
		Where("revoked_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "revoke all operator mfa trusted devices", Err: err}
	}
	return nil
}

func (r *OperatorMFATrustedDeviceRepository) DeleteExpired(ctx context.Context) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*platform.OperatorMFATrustedDevice)(nil)).
		ModelTableExpr(operatorMFATrustedDeviceTable).
		Where("expires_at < ?", time.Now()).
		Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete expired operator mfa trusted devices", Err: err}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "rows affected for delete expired operator mfa trusted devices", Err: err}
	}
	return int(affected), nil
}
