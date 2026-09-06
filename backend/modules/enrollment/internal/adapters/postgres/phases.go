package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

const phaseTableExpr = `enrollment.phases AS "phase"`

func (r *Store) OpenPhaseCandidates(ctx context.Context) ([]*enrollment.Phase, error) {
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*phaseRow
	err = db.NewSelect().Model(&rows).
		Where(`"phase".is_active = TRUE`).
		Where(`("phase".enrollment_open_at IS NULL OR "phase".enrollment_open_at <= NOW())`).
		Where(`("phase".enrollment_close_at IS NULL OR "phase".enrollment_close_at >= NOW())`).
		OrderExpr(`"phase".service_start_date ASC`).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollable phases: %w", err)
	}
	return phaseValues(rows), nil
}

func (r *Store) InsertPhase(ctx context.Context, phase *enrollment.Phase) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("phase validation: %w", err)
	}
	phase.TenantID = tenantID
	if err := r.checkPhaseSchema(ctx, phase.FormSchemaID); err != nil {
		return err
	}
	if phase.RolloverSourcePhaseID != nil {
		if _, err := r.Phase(ctx, *phase.RolloverSourcePhaseID); err != nil {
			return fmt.Errorf("load rollover source phase: %w", err)
		}
	}

	row := phaseRecord(phase)
	_, err = db.NewInsert().
		Model(row).
		ModelTableExpr(phaseTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create phase: %w", err)
	}
	*phase = *row.value()
	return nil
}

func (r *Store) Phase(ctx context.Context, id int64) (*enrollment.Phase, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	row := new(phaseRow)
	err = db.NewSelect().
		Model(row).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("phase %d not found: %w", id, err)
		}
		return nil, fmt.Errorf("failed to find phase: %w", err)
	}
	return row.value(), nil
}

func (r *Store) PhasesByID(ctx context.Context, ids []int64) ([]*enrollment.Phase, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []*phaseRow
	if err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list phases by ids: %w", err)
	}
	return phaseValues(rows), nil
}

func (r *Store) UpdatePhase(ctx context.Context, phase *enrollment.Phase) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("phase validation: %w", err)
	}
	if phase.ID == 0 {
		return errors.New("phase ID is required for update")
	}
	if err := r.checkPhaseSchema(ctx, phase.FormSchemaID); err != nil {
		return err
	}

	res, err := db.NewUpdate().
		Model(phaseRecord(phase)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
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
	rows, countErr := res.RowsAffected()
	if countErr != nil {
		return fmt.Errorf("phase affected rows: %w", countErr)
	}
	if rows == 0 {
		return fmt.Errorf("phase %d not found: %w", phase.ID, sql.ErrNoRows)
	}
	return nil
}

func (r *Store) checkPhaseSchema(ctx context.Context, schemaID *int64) error {
	if schemaID == nil {
		return nil
	}
	if _, err := r.Schema(ctx, *schemaID); err != nil {
		return fmt.Errorf("load phase form schema: %w", err)
	}
	return nil
}

func (r *Store) DeletePhase(ctx context.Context, id int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewDelete().
		Model((*phaseRow)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete phase: %w", err)
	}
	rows, countErr := res.RowsAffected()
	if countErr != nil {
		return fmt.Errorf("phase affected rows: %w", countErr)
	}
	if rows == 0 {
		return fmt.Errorf("phase %d not found", id)
	}
	return nil
}

func (r *Store) Phases(ctx context.Context) ([]*enrollment.Phase, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*phaseRow
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		OrderExpr(`"phase".service_start_date DESC, "phase".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases: %w", err)
	}
	return phaseValues(rows), nil
}

func (r *Store) PublicOpenPhases(ctx context.Context, now time.Time) ([]*enrollment.Phase, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*phaseRow
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
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
	return phaseValues(rows), nil
}

func (r *Store) PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*enrollment.Phase, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*phaseRow
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".rollover_deadline IS NOT NULL`).
		Where(`"phase".rollover_deadline <= ?`, asOf).
		OrderExpr(`"phase".rollover_deadline ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list phases with expired rollover deadline: %w", err)
	}
	return phaseValues(rows), nil
}

func (r *Store) HasActiveClassRestrictedPhase(ctx context.Context) (bool, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}
	exists, err := db.NewSelect().
		Model((*phaseRow)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".is_active = TRUE`).
		Where(`EXISTS (SELECT 1 FROM jsonb_array_elements_text("phase".eligible_school_classes) AS elem WHERE btrim(elem) <> '')`).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check active phases with eligible classes: %w", err)
	}
	return exists, nil
}

func (r *Store) HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}
	exists, err := db.NewSelect().
		Model((*phaseRow)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".is_active = TRUE`).
		Where(`jsonb_array_length("phase".eligible_grade_levels) > 0`).
		Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check active phases with eligible grade levels: %w", err)
	}
	return exists, nil
}

func (r *Store) MaxActivePhaseGrade(ctx context.Context) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	var highest int
	err = db.NewSelect().
		Model((*phaseRow)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		ColumnExpr(`COALESCE(MAX((grade.value #>> '{}')::int), 0)`).
		Join(`CROSS JOIN LATERAL jsonb_array_elements("phase".eligible_grade_levels) AS grade(value)`).
		Where(`"phase".is_active = TRUE`).
		Scan(ctx, &highest)
	if err != nil {
		return 0, fmt.Errorf("failed to read highest active eligible grade level: %w", err)
	}
	return highest, nil
}

func (r *Store) HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}
	count, err := db.NewSelect().
		Model((*phaseRow)(nil)).
		ModelTableExpr(phaseTableExpr).
		Where(`"phase".tenant_id = ?`, tenantID).
		Where(`"phase".rollover_source_phase_id = ?`, sourcePhaseID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check rollover source references: %w", err)
	}
	return count > 0, nil
}
