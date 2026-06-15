package enrollment

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestGuardianTableExpr = `enrollment.request_guardians AS "request_guardian"`

type RequestGuardianRepository struct {
	db *bun.DB
}

func NewRequestGuardianRepository(db *bun.DB) enrollment.RequestGuardianRepository {
	return &RequestGuardianRepository{db: db}
}

func (r *RequestGuardianRepository) Create(ctx context.Context, guardian *enrollment.RequestGuardian) error {
	base.EnsureTenantID(ctx, guardian)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(guardian).
		ModelTableExpr(requestGuardianTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request guardian: %w", err)
	}
	return nil
}

// ListByRequestID returns all additional guardians for a request, sorted
// by sort_order. Tenant-scoped via RLS.
func (r *RequestGuardianRepository) ListByRequestID(ctx context.Context, requestID int64) ([]*enrollment.RequestGuardian, error) {
	var guardians []*enrollment.RequestGuardian
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&guardians).
		ModelTableExpr(requestGuardianTableExpr).
		Where(`"request_guardian".request_id = ?`, requestID).
		OrderExpr(`"request_guardian".sort_order, "request_guardian".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request guardians: %w", err)
	}
	return guardians, nil
}

// StampResolvedProfile back-links the co-guardian row to the
// guardian_profiles row it resolved to on the first child approval.
func (r *RequestGuardianRepository) StampResolvedProfile(ctx context.Context, id, guardianProfileID int64) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestGuardian)(nil)).
		ModelTableExpr(requestGuardianTableExpr).
		Set("guardian_profile_id = ?", guardianProfileID).
		Set("updated_at = ?", time.Now()).
		Where(`"request_guardian".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to stamp resolved guardian profile: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request guardian %d not found", id)
	}
	return nil
}
