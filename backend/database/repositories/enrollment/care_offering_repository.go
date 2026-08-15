package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
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
			return nil, fmt.Errorf("care offering %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find care offering: %w", err)
	}
	if err := r.hydrateAutoAddTriggers(ctx, []*enrollment.CareOffering{offering}); err != nil {
		return nil, err
	}
	return offering, nil
}

// Update writes a full row to disk. Every editable column is set
// explicitly — chaining a single .Set("updated_at = NOW()") onto a
// Model() update makes bun emit ONLY that SET, which silently dropped
// every other column change before the phase model rewrite.
//
// Concurrency is naive — first writer wins. Acceptable for an admin
// catalog editor with single-editor expectations.
func (r *CareOfferingRepository) Update(ctx context.Context, offering *enrollment.CareOffering) error {
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(offering).
		ModelTableExpr(careOfferingTableExpr).
		Set("phase_id = ?", offering.PhaseID).
		Set("activity_group_id = ?", offering.ActivityGroupID).
		Set("name = ?", offering.Name).
		Set("description = ?", offering.Description).
		Set("days_of_week_mode = ?", offering.DaysOfWeekMode).
		Set("available_days = ?", offering.AvailableDays).
		Set("includes_holiday_care = ?", offering.IncludesHolidayCare).
		Set("includes_lunch = ?", offering.IncludesLunch).
		Set("capacity = ?", offering.Capacity).
		Set("price_cents = ?", offering.PriceCents).
		Set("is_active = ?", offering.IsActive).
		Set("is_required = ?", offering.IsRequired).
		Set("counts_as_care = ?", offering.CountsAsCare).
		Set("auto_add_grade_levels = ?", offering.AutoAddGradeLevels).
		Set("availability_rule = ?", offering.AvailabilityRule).
		Set("selection_group = ?", offering.SelectionGroup).
		Set("selection_rule = ?", offering.SelectionRule).
		Set("sort_order = ?", offering.SortOrder).
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

func (r *CareOfferingRepository) ReplaceAutoAddTriggers(ctx context.Context, targetOfferingID int64, triggerOfferingIDs []int64) error {
	if targetOfferingID <= 0 {
		return fmt.Errorf("target offering id must be positive")
	}
	db := base.GetDB(ctx, r.db)
	if _, err := db.NewDelete().
		Model((*enrollment.CareOfferingAutoTrigger)(nil)).
		ModelTableExpr(`enrollment.care_offering_auto_triggers AS "care_offering_auto_trigger"`).
		Where(`"care_offering_auto_trigger".target_care_offering_id = ?`, targetOfferingID).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete care offering auto triggers: %w", err)
	}
	if len(triggerOfferingIDs) == 0 {
		return nil
	}
	rows := make([]*enrollment.CareOfferingAutoTrigger, 0, len(triggerOfferingIDs))
	seen := make(map[int64]bool, len(triggerOfferingIDs))
	for _, triggerID := range triggerOfferingIDs {
		if triggerID <= 0 || triggerID == targetOfferingID || seen[triggerID] {
			continue
		}
		seen[triggerID] = true
		row := &enrollment.CareOfferingAutoTrigger{
			TargetCareOfferingID:  targetOfferingID,
			TriggerCareOfferingID: triggerID,
		}
		base.EnsureTenantID(ctx, row)
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr(`enrollment.care_offering_auto_triggers AS "care_offering_auto_trigger"`).
		Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert care offering auto triggers: %w", err)
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
	if err := r.hydrateAutoAddTriggers(ctx, offerings); err != nil {
		return nil, err
	}
	return offerings, nil
}

// ListByPhase returns every care offering for a given phase, sorted
// by sort_order. Admin pages use this once a phase is selected.
func (r *CareOfferingRepository) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollment.CareOffering, error) {
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".phase_id = ?`, phaseID).
		OrderExpr(`"care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings by phase: %w", err)
	}
	if err := r.hydrateAutoAddTriggers(ctx, offerings); err != nil {
		return nil, err
	}
	return offerings, nil
}

// ListByIDs returns the exact care offerings referenced by ids. It is not
// phase-scoped: callers like the decision path resolve whatever IDs are
// persisted on a request child, and legacy pre-#2249 rollover rows may
// still carry source-phase offering IDs.
func (r *CareOfferingRepository) ListByIDs(ctx context.Context, ids []int64) ([]*enrollment.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollment.CareOffering{}, nil
	}
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".id IN (?)`, bun.List(ids)).
		OrderExpr(`"care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings by ids: %w", err)
	}
	if err := r.hydrateAutoAddTriggers(ctx, offerings); err != nil {
		return nil, err
	}
	return offerings, nil
}

// ListByIDsForUpdate loads and locks the requested offerings in a stable order.
// The lock is held by the surrounding tenant transaction and serializes the
// capacity check with the booking mutation of competing approvals.
func (r *CareOfferingRepository) ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollment.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollment.CareOffering{}, nil
	}
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".id IN (?)`, bun.List(ids)).
		OrderExpr(`"care_offering".id`).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to lock care offerings by ids: %w", err)
	}
	if err := r.hydrateAutoAddTriggers(ctx, offerings); err != nil {
		return nil, err
	}
	return offerings, nil
}

func (r *CareOfferingRepository) ListByActivityGroupIDs(
	ctx context.Context,
	activityGroupIDs []int64,
) ([]*enrollment.CareOffering, error) {
	if len(activityGroupIDs) == 0 {
		return []*enrollment.CareOffering{}, nil
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is required to list care offerings by activity groups")
	}
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".tenant_id = ?`, tenantID).
		Where(`"care_offering".activity_group_id IN (?)`, bun.List(activityGroupIDs)).
		OrderExpr(`"care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings by activity groups: %w", err)
	}
	return offerings, nil
}

// CountByPhaseID returns how many care offerings belong to the phase.
// Powers the phase-delete confirmation modal ("Y Betreuungsangebote
// werden gelöscht"). Tenant-scoped via RLS.
func (r *CareOfferingRepository) CountByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.CareOffering)(nil)).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".phase_id = ?`, phaseID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count care offerings by phase: %w", err)
	}
	return count, nil
}

// ListActiveByPhase returns is_active=true offerings for the phase.
// The phase's enrollment window is enforced one layer up (handler
// rejects the parent before this query runs), so this method only
// has to filter on the soft-disable flag.
func (r *CareOfferingRepository) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollment.CareOffering, error) {
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".phase_id = ?`, phaseID).
		Where(`"care_offering".is_active = TRUE`).
		OrderExpr(`"care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active offerings by phase: %w", err)
	}
	if err := r.hydrateAutoAddTriggers(ctx, offerings); err != nil {
		return nil, err
	}
	return offerings, nil
}

func (r *CareOfferingRepository) ListActiveByPhaseIDs(ctx context.Context, phaseIDs []int64) ([]*enrollment.CareOffering, error) {
	if len(phaseIDs) == 0 {
		return nil, nil
	}
	var offerings []*enrollment.CareOffering
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&offerings).
		ModelTableExpr(careOfferingTableExpr).
		Where(`"care_offering".phase_id IN (?)`, bun.List(phaseIDs)).
		Where(`"care_offering".is_active = TRUE`).
		OrderExpr(`"care_offering".phase_id, "care_offering".sort_order, "care_offering".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list active care offerings by phase ids: %w", err)
	}
	if err := r.hydrateAutoAddTriggers(ctx, offerings); err != nil {
		return nil, err
	}
	return offerings, nil
}

func (r *CareOfferingRepository) hydrateAutoAddTriggers(ctx context.Context, offerings []*enrollment.CareOffering) error {
	if len(offerings) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(offerings))
	byID := make(map[int64]*enrollment.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering == nil || offering.ID <= 0 {
			continue
		}
		offering.CountsAsCareSet = true
		offering.AutoAddTriggerOfferingIDs = []int64{}
		ids = append(ids, offering.ID)
		byID[offering.ID] = offering
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []*enrollment.CareOfferingAutoTrigger
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`enrollment.care_offering_auto_triggers AS "care_offering_auto_trigger"`).
		Where(`"care_offering_auto_trigger".target_care_offering_id IN (?)`, bun.List(ids)).
		OrderExpr(`"care_offering_auto_trigger".target_care_offering_id, "care_offering_auto_trigger".trigger_care_offering_id`).
		Scan(ctx)
	if err != nil {
		return fmt.Errorf("failed to load care offering auto triggers: %w", err)
	}
	for _, row := range rows {
		if offering := byID[row.TargetCareOfferingID]; offering != nil {
			offering.AutoAddTriggerOfferingIDs = append(offering.AutoAddTriggerOfferingIDs, row.TriggerCareOfferingID)
		}
	}
	return nil
}
