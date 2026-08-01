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

const phaseTableExpr = `enrollment.phases AS "phase"`

// PhaseRepository implements enrollment.PhaseRepository against the
// enrollment.phases table. RLS-scoped via tenant context; the public
// "list open" path uses an admin tx wrapper at the handler layer.
type PhaseRepository struct {
	db *bun.DB
}

func NewPhaseRepository(db *bun.DB) enrollment.PhaseRepository {
	return &PhaseRepository{db: db}
}

func (r *PhaseRepository) Create(ctx context.Context, phase *enrollment.Phase) error {
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("phase validation: %w", err)
	}
	base.EnsureTenantID(ctx, phase)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(phase).
		ModelTableExpr(phaseTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create phase: %w", err)
	}
	return nil
}

func (r *PhaseRepository) FindByID(ctx context.Context, id int64) (*enrollment.Phase, error) {
	row := new(enrollment.Phase)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("phase %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find phase: %w", err)
	}
	return row, nil
}

func (r *PhaseRepository) ListByIDs(ctx context.Context, ids []int64) ([]*enrollment.Phase, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []*enrollment.Phase
	if err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list phases by ids: %w", err)
	}
	return rows, nil
}

func (r *PhaseRepository) Update(ctx context.Context, phase *enrollment.Phase) error {
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("phase validation: %w", err)
	}
	if phase.ID == 0 {
		return errors.New("phase ID is required for update")
	}

	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model(phase).
		ModelTableExpr(phaseTableExpr).
		Set("name = ?", phase.Name).
		Set("kind = ?", phase.Kind).
		Set("service_start_date = ?", phase.ServiceStartDate).
		Set("service_end_date = ?", phase.ServiceEndDate).
		Set("enrollment_open_at = ?", phase.EnrollmentOpenAt).
		Set("enrollment_close_at = ?", phase.EnrollmentCloseAt).
		Set("form_schema_id = ?", phase.FormSchemaID).
		Set("calendar_period_id = ?", phase.CalendarPeriodID).
		Set("show_status_reason_to_parent = ?", phase.ShowStatusReasonToParent).
		Set("care_overflow_mode = ?", phase.CareOverflowMode).
		Set("care_offering_selection_mode = ?", phase.CareOfferingSelectionMode).
		Set("is_active = ?", phase.IsActive).
		Set("available_school_classes = ?", phase.AvailableSchoolClasses).
		Set("require_school_class = ?", phase.RequireSchoolClass).
		Set("audience = ?", phase.Audience).
		Set("eligible_school_classes = ?", phase.EligibleSchoolClasses).
		Set("eligible_grade_levels = ?", phase.EligibleGradeLevels).
		Set("updated_at = NOW()").
		Where(`"phase".id = ?`, phase.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update phase: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("phase %d not found: %w", phase.ID, sql.ErrNoRows)
	}
	return nil
}

func (r *PhaseRepository) Delete(ctx context.Context, id int64) error {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete phase: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("phase %d not found", id)
	}
	return nil
}

func (r *PhaseRepository) ListByTenant(ctx context.Context) ([]*enrollment.Phase, error) {
	var rows []*enrollment.Phase
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		OrderExpr(`"phase".service_start_date DESC, "phase".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases: %w", err)
	}
	return rows, nil
}

// ListPublicOpen filters to is_active=true AND the open window
// includes `now`. We push the filter down to SQL so the parent-facing
// path doesn't pull every phase into memory; NULL bounds are treated
// as "unbounded" via the IS NULL OR ... pattern.
//
// Audience-restricted phases (linked_parents AND existing_students) are
// excluded entirely: their eligibility cannot be verified for an anonymous
// visitor, so they are only reachable through the authenticated parents
// portal (#1663). The excluded set must stay identical to the anonymous
// form-load gate (services/enrollment.audienceRequiresGuardianAccount) —
// advertising a card whose form the very next request 404s is a guaranteed
// dead end for the parent.
func (r *PhaseRepository) ListPublicOpen(ctx context.Context, now time.Time) ([]*enrollment.Phase, error) {
	var rows []*enrollment.Phase
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".is_active = TRUE`).
		Where(`"phase".audience NOT IN (?)`, bun.List([]string{
			enrollment.PhaseAudienceLinkedParents,
			enrollment.PhaseAudienceExistingStudents,
		})).
		Where(`("phase".enrollment_open_at IS NULL OR "phase".enrollment_open_at <= ?)`, now).
		Where(`("phase".enrollment_close_at IS NULL OR "phase".enrollment_close_at > ?)`, now).
		OrderExpr(`"phase".service_start_date ASC, "phase".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list public open phases: %w", err)
	}
	return rows, nil
}

// ListWithExpiredRolloverDeadline returns rollover phases whose
// deadline has passed and that therefore still have renewal-pending
// children waiting on the scheduled resolver. Filters are pushed to
// SQL so the worker doesn't pull every phase into memory.
func (r *PhaseRepository) ListWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*enrollment.Phase, error) {
	var rows []*enrollment.Phase
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".rollover_deadline IS NOT NULL`).
		Where(`"phase".rollover_deadline <= ?`, asOf).
		OrderExpr(`"phase".rollover_deadline ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases with expired rollover deadline: %w", err)
	}
	return rows, nil
}

// ExistsActiveWithEligibleClasses reports whether the tenant (via the RLS
// tx in context) has any active phase whose eligible_school_classes carries
// at least one non-blank entry. The blank filter mirrors the model's
// hasNonEmptyEligibleClass so a stored [""] is not treated as a real
// restriction. Backs the settings guard that refuses to disable
// concrete-class collection while such a phase exists (#1663).
func (r *PhaseRepository) ExistsActiveWithEligibleClasses(ctx context.Context) (bool, error) {
	exists, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".is_active = TRUE`).
		Where(`EXISTS (SELECT 1 FROM jsonb_array_elements_text("phase".eligible_school_classes) AS elem WHERE btrim(elem) <> '')`).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check active phases with eligible classes: %w", err)
	}
	return exists, nil
}

// ExistsActiveWithEligibleGradeLevels reports whether the tenant (via the
// RLS tx in context) has any active phase whose eligible_grade_levels holds
// at least one entry. Grades are stored as JSON numbers written only through
// Phase.Validate, so a length check is exact — there is no blank-entry case
// like the class list has. Backs the settings guard that refuses to disable
// grade-level collection while such a phase exists (#1663).
func (r *PhaseRepository) ExistsActiveWithEligibleGradeLevels(ctx context.Context) (bool, error) {
	exists, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".is_active = TRUE`).
		Where(`jsonb_array_length("phase".eligible_grade_levels) > 0`).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check active phases with eligible grade levels: %w", err)
	}
	return exists, nil
}

// MaxActiveEligibleGradeLevel returns the highest grade any active phase of
// the tenant (via the RLS tx in context) restricts itself to, or 0 when no
// active phase carries a grade restriction. Backs the settings guard that
// refuses to lower enrollment.grade_level_max below a restriction that is
// already live — the form would then offer no eligible grade at all and every
// submission to that phase would fail (#1663). Grades are stored as JSON
// numbers written only through Phase.Validate, so the text-extract cast is
// safe.
func (r *PhaseRepository) MaxActiveEligibleGradeLevel(ctx context.Context) (int, error) {
	var highest int
	err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		ColumnExpr(`COALESCE(MAX((grade.value #>> '{}')::int), 0)`).
		Join(`CROSS JOIN LATERAL jsonb_array_elements("phase".eligible_grade_levels) AS grade(value)`).
		Where(`"phase".is_active = TRUE`).
		Scan(ctx, &highest)
	if err != nil {
		return 0, fmt.Errorf("failed to read highest active eligible grade level: %w", err)
	}
	return highest, nil
}

// ExistsByFormSchemaID is the safety check the schema-delete path needs.
// Returns true if any phase still references the given schema id.
func (r *PhaseRepository) ExistsByFormSchemaID(ctx context.Context, schemaID int64) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".form_schema_id = ?`, schemaID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check phase references: %w", err)
	}
	return count > 0, nil
}

// RepointFormSchema advances every phase currently bound to one of
// fromIDs so it points at toID instead. Used when a schema is edited:
// publishing a new version mints a new schema id, and live phases must
// follow it so the admin's change reaches the public form without a
// manual re-bind. Already-submitted requests keep their own pinned
// schema_id (copied at submit time), so history is unaffected. Returns
// the number of phases updated. Tenant-scoped via the request's RLS tx.
func (r *PhaseRepository) RepointFormSchema(ctx context.Context, fromIDs []int64, toID int64) (int64, error) {
	if len(fromIDs) == 0 {
		return 0, nil
	}
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		Set("form_schema_id = ?", toID).
		Set("updated_at = NOW()").
		Where(`"phase".form_schema_id IN (?)`, bun.List(fromIDs)).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to repoint phases to schema %d: %w", toID, err)
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// ExistsByRolloverSourcePhaseID is the rollover-uniqueness check.
// Returns true if any phase already references the given phase as its
// rollover_source_phase_id — i.e., the source has been rolled forward
// once already and can't be rolled again.
func (r *PhaseRepository) ExistsByRolloverSourcePhaseID(ctx context.Context, sourcePhaseID int64) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Phase)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".rollover_source_phase_id = ?`, sourcePhaseID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check rollover source references: %w", err)
	}
	return count > 0, nil
}
