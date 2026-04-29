package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const careOfferingTableExpr = `enrollment.care_offerings AS "care_offering"`

// CareOfferingRepository is the bun-backed implementation of
// enrollment.CareOfferingRepository.
type CareOfferingRepository struct {
	db *bun.DB
}

func NewCareOfferingRepository(db *bun.DB) enrollment.CareOfferingRepository {
	return &CareOfferingRepository{db: db}
}

// Create inserts a new care offering. Tenant ID auto-populated from
// the transaction context.
func (r *CareOfferingRepository) Create(ctx context.Context, offering *enrollment.CareOffering) error {
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	base.EnsureTenantID(ctx, offering)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(offering).
		ModelTableExpr(careOfferingTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create care offering: %w", err)
	}
	return nil
}

func (r *CareOfferingRepository) FindByID(ctx context.Context, id int64) (*enrollment.CareOffering, error) {
	offering := new(enrollment.CareOffering)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(offering).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("care offering %d not found", id)
		}
		return nil, fmt.Errorf("failed to find care offering: %w", err)
	}
	return offering, nil
}

// Update writes a full row to disk. Concurrency is naive — first
// writer wins. Acceptable for an admin catalog editor with single-
// editor expectations.
func (r *CareOfferingRepository) Update(ctx context.Context, offering *enrollment.CareOffering) error {
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(offering).
		ModelTableExpr(careOfferingTableExpr).
		Set("updated_at = NOW()").
		Where(`"care_offering".id = ?`, offering.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update care offering: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("care offering %d not found", offering.ID)
	}
	return nil
}

// Delete removes the row. The FK from request_child_offerings is ON
// DELETE RESTRICT, so this fails when any submission already
// references the offering — admin should soft-delete via is_active=false
// in that case.
func (r *CareOfferingRepository) Delete(ctx context.Context, id int64) error {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.CareOffering)(nil)).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete care offering: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("care offering %d not found", id)
	}
	return nil
}

func (r *CareOfferingRepository) ListByTenant(ctx context.Context) ([]*enrollment.CareOffering, error) {
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		OrderExpr(`"care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings: %w", err)
	}
	return offerings, nil
}

func (r *CareOfferingRepository) ListByCalendarPeriod(ctx context.Context, calendarPeriodID int64) ([]*enrollment.CareOffering, error) {
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".calendar_period_id = ?`, calendarPeriodID).
		OrderExpr(`"care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings by period: %w", err)
	}
	return offerings, nil
}

// ListPublicOpenWindow returns offerings the public form should expose:
// is_active=true AND `now` falls within (or NULL bounds replace) the
// application window. NULL bounds mean unbounded — passing those rows
// through is intentional.
func (r *CareOfferingRepository) ListPublicOpenWindow(ctx context.Context, now time.Time) ([]*enrollment.CareOffering, error) {
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".is_active = TRUE`).
		Where(`("care_offering".application_window_start IS NULL OR "care_offering".application_window_start <= ?)`, now).
		Where(`("care_offering".application_window_end IS NULL OR "care_offering".application_window_end > ?)`, now).
		OrderExpr(`"care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list open-window care offerings: %w", err)
	}
	return offerings, nil
}
