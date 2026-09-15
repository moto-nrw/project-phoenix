package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type requestGuardianRow struct {
	bun.BaseModel     `bun:"table:enrollment.request_guardians,alias:request_guardian"`
	ID                int64     `bun:"id,pk,autoincrement"`
	TenantID          int64     `bun:"tenant_id,notnull"`
	CreatedAt         time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	RequestID         int64     `bun:"request_id,notnull"`
	FirstName         string    `bun:"first_name,notnull"`
	LastName          string    `bun:"last_name,notnull"`
	Email             *string   `bun:"email"`
	Phone             *string   `bun:"phone"`
	GuardianProfileID *int64    `bun:"guardian_profile_id"`
	SortOrder         int       `bun:"sort_order,notnull,default:0"`
}

func (row requestGuardianRow) value() *enrollment.RequestGuardian {
	return &enrollment.RequestGuardian{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, RequestID: row.RequestID, FirstName: row.FirstName, LastName: row.LastName, Email: row.Email, Phone: row.Phone, GuardianProfileID: row.GuardianProfileID, SortOrder: row.SortOrder}
}
func (r *Store) CreateRequestGuardian(ctx context.Context, guardian *enrollment.RequestGuardian) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	if guardian.TenantID != 0 && guardian.TenantID != tenantID {
		return fmt.Errorf("request guardian tenant mismatch")
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	exists, err := db.NewSelect().TableExpr("enrollment.requests").Where("id = ? AND tenant_id = ?", guardian.RequestID, tenantID).Exists(ctx)
	if err != nil {
		return fmt.Errorf("find guardian request: %w", err)
	}
	if !exists {
		return fmt.Errorf("request not found")
	}
	row := &requestGuardianRow{ID: guardian.ID, TenantID: guardian.TenantID, CreatedAt: guardian.CreatedAt, UpdatedAt: guardian.UpdatedAt, RequestID: guardian.RequestID, FirstName: guardian.FirstName, LastName: guardian.LastName, Email: guardian.Email, Phone: guardian.Phone, GuardianProfileID: guardian.GuardianProfileID, SortOrder: guardian.SortOrder}
	row.TenantID = tenantID
	_, err = db.NewInsert().Model(row).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request guardian: %w", err)
	}
	*guardian = *row.value()
	return nil
}
func (r *Store) RequestGuardians(ctx context.Context, ids []int64) ([]*enrollment.RequestGuardian, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestGuardianRow
	err = db.NewSelect().Model(&rows).Where("request_guardian.tenant_id = ?", tenantID).Where("request_guardian.request_id IN (?)", bun.List(ids)).OrderExpr("request_guardian.request_id, request_guardian.sort_order, request_guardian.id").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request guardians: %w", err)
	}
	var result []*enrollment.RequestGuardian
	for _, row := range rows {
		result = append(result, row.value())
	}
	return result, nil
}
func (r *Store) DeleteRequestGuardians(ctx context.Context, requestID int64) error {
	if requestID <= 0 {
		return fmt.Errorf("request id must be positive")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().Model((*requestGuardianRow)(nil)).Where("request_guardian.tenant_id = ?", tenantID).Where("request_guardian.request_id = ?", requestID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete request guardians: %w", err)
	}
	return nil
}
func (r *Store) StampRequestGuardianProfile(ctx context.Context, id, profileID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().Model((*requestGuardianRow)(nil)).Set("guardian_profile_id = ?", profileID).Set("updated_at = ?", time.Now()).Where("request_guardian.tenant_id = ?", tenantID).Where("request_guardian.id = ?", id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to stamp resolved guardian profile: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request guardian %d not found", id)
	}
	return nil
}
