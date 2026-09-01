package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

const (
	tableWorkTimeModels       = "config.work_time_models"
	tableWorkTimeModelEntries = "config.work_time_model_entries"
)

// WorkTimeModelRepository implements config.WorkTimeModelRepository on top of bun.
type WorkTimeModelRepository struct {
	runtime Runtime
}

// NewWorkTimeModelRepository wires the bun-backed implementation.
func NewWorkTimeModelRepository(runtime Runtime) config.WorkTimeModelRepository {
	return &WorkTimeModelRepository{runtime: runtime}
}

// List returns every template visible to the active tenant, eagerly loading entries.
//
// Entries are loaded with an explicit second SELECT instead of bun's
// Relation() helper because the latter dropped the schema qualifier on the
// FROM clause and crashed with "relation \"work_time_model_entries\" does
// not exist". Two queries is fine for tenant-scoped template lists.
func (r *WorkTimeModelRepository) List(ctx context.Context) ([]*config.WorkTimeModel, error) {
	var models []*config.WorkTimeModel
	query := r.runtime.DB(ctx).NewSelect().
		Model(&models).
		ModelTableExpr(tableWorkTimeModels + ` AS "work_time_model"`).
		OrderExpr(`"work_time_model".name ASC`)

	if tenantID := r.runtime.TenantID(ctx); tenantID > 0 {
		query = query.Where(`"work_time_model".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list work-time models: %w", err)
	}
	return r.attachEntries(ctx, models)
}

// FindByID resolves a single template with its entries; returns sql.ErrNoRows when missing.
func (r *WorkTimeModelRepository) FindByID(ctx context.Context, id int64) (*config.WorkTimeModel, error) {
	model := new(config.WorkTimeModel)
	query := r.runtime.DB(ctx).NewSelect().
		Model(model).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		Where(`"work_time_model".id = ?`, id)

	if tenantID := r.runtime.TenantID(ctx); tenantID > 0 {
		query = query.Where(`"work_time_model".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, err
	}

	enriched, err := r.attachEntries(ctx, []*config.WorkTimeModel{model})
	if err != nil {
		return nil, err
	}
	return enriched[0], nil
}

// FindByIDs resolves the given templates with their entries; IDs that do not
// exist (or belong to another tenant) are absent from the result.
func (r *WorkTimeModelRepository) FindByIDs(ctx context.Context, ids []int64) ([]*config.WorkTimeModel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var models []*config.WorkTimeModel
	query := r.runtime.DB(ctx).NewSelect().
		Model(&models).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		Where(`"work_time_model".id IN (?)`, bun.List(ids))

	if tenantID := r.runtime.TenantID(ctx); tenantID > 0 {
		query = query.Where(`"work_time_model".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("find work-time models by ids: %w", err)
	}
	return r.attachEntries(ctx, models)
}

func (r *WorkTimeModelRepository) attachEntries(ctx context.Context, models []*config.WorkTimeModel) ([]*config.WorkTimeModel, error) {
	if len(models) == 0 {
		return models, nil
	}
	ids := make([]int64, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	var entries []*config.WorkTimeModelEntry
	if err := r.runtime.DB(ctx).NewSelect().
		Model(&entries).
		ModelTableExpr(tableWorkTimeModelEntries+` AS "work_time_model_entry"`).
		Where(`"work_time_model_entry".model_id IN (?)`, bun.List(ids)).
		OrderExpr(`"work_time_model_entry".week_index ASC, "work_time_model_entry".day_of_week ASC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	byModel := make(map[int64][]*config.WorkTimeModelEntry, len(models))
	for _, e := range entries {
		byModel[e.ModelID] = append(byModel[e.ModelID], e)
	}
	for _, m := range models {
		m.Entries = byModel[m.ID]
	}
	return models, nil
}

// Create inserts the template and its entries inside one transaction.
func (r *WorkTimeModelRepository) Create(ctx context.Context, model *config.WorkTimeModel, entries []*config.WorkTimeModelEntry) error {
	db := r.runtime.DB(ctx)
	tenantID := r.runtime.TenantID(ctx)
	if tenantID > 0 {
		model.SetTenantID(tenantID)
	}
	if err := model.Validate(); err != nil {
		return fmt.Errorf("invalid model: %w", err)
	}
	if err := validateEntriesForRotation(entries, model.RotationLength); err != nil {
		return err
	}

	if _, err := db.NewInsert().Model(model).ModelTableExpr(tableWorkTimeModels).Exec(ctx); err != nil {
		return fmt.Errorf("insert model: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	for _, e := range entries {
		e.ModelID = model.ID
	}
	if _, err := db.NewInsert().Model(&entries).ModelTableExpr(tableWorkTimeModelEntries).Exec(ctx); err != nil {
		return fmt.Errorf("insert entries: %w", err)
	}
	return nil
}

// Update replaces the template's metadata + every entry atomically.
func (r *WorkTimeModelRepository) Update(ctx context.Context, model *config.WorkTimeModel, entries []*config.WorkTimeModelEntry) error {
	db := r.runtime.DB(ctx)
	tenantID := r.runtime.TenantID(ctx)
	if tenantID > 0 {
		model.SetTenantID(tenantID)
	}
	if err := model.Validate(); err != nil {
		return fmt.Errorf("invalid model: %w", err)
	}
	if err := validateEntriesForRotation(entries, model.RotationLength); err != nil {
		return err
	}

	updateQuery := db.NewUpdate().
		Model(model).
		ModelTableExpr(tableWorkTimeModels).
		Set("name = ?", model.Name).
		Set("rotation_length = ?", model.RotationLength).
		Set("rotation_anchor_date = ?", model.RotationAnchorDate).
		Set("updated_at = NOW()").
		Where("id = ?", model.ID)
	if tenantID > 0 {
		updateQuery = updateQuery.Where("tenant_id = ?", tenantID)
	}
	res, err := updateQuery.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update model: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update model rows affected: %w", err)
	}
	if rows != 1 {
		return sql.ErrNoRows
	}

	if _, err := db.NewDelete().
		Model((*config.WorkTimeModelEntry)(nil)).
		ModelTableExpr(tableWorkTimeModelEntries).
		Where("model_id = ?", model.ID).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete old entries: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}
	for _, e := range entries {
		e.ModelID = model.ID
	}
	if _, err := db.NewInsert().Model(&entries).ModelTableExpr(tableWorkTimeModelEntries).Exec(ctx); err != nil {
		return fmt.Errorf("insert entries: %w", err)
	}
	return nil
}

func (r *WorkTimeModelRepository) RefreshAssignedStaffSchedules(ctx context.Context, modelID int64) error {
	model, err := r.FindByID(ctx, modelID)
	if err != nil {
		return fmt.Errorf("load model for assigned schedule refresh: %w", err)
	}

	db := r.runtime.DB(ctx)
	tenantID := r.runtime.TenantID(ctx)
	var staffIDs []int64
	assignedQuery := db.NewSelect().
		TableExpr(`users.staff AS "staff"`).
		ColumnExpr(`"staff".id`).
		Where(`"staff".work_time_model_id = ?`, modelID).
		Where(`"staff".deleted_at IS NULL`)
	if tenantID > 0 {
		assignedQuery = assignedQuery.Where(`"staff".tenant_id = ?`, tenantID)
	}
	if err := assignedQuery.Scan(ctx, &staffIDs); err != nil {
		return fmt.Errorf("load assigned staff: %w", err)
	}
	if len(staffIDs) == 0 {
		return nil
	}
	slices.Sort(staffIDs)
	for _, staffID := range staffIDs {
		if err := r.runtime.LockStaffBalance(ctx, staffID); err != nil {
			return err
		}
	}

	now := time.Now()
	today := config.CalendarDateFromTime(r.runtime.TodayTime())
	closeQuery := db.NewUpdate().
		TableExpr(tableStaffWorkSchedules).
		Set("valid_until = ?", today).
		Set("updated_at = ?", now).
		Where("staff_id IN (?)", bun.List(staffIDs)).
		Where("valid_until IS NULL")
	if tenantID > 0 {
		closeQuery = closeQuery.Where("tenant_id = ?", tenantID)
	}
	if _, err := closeQuery.Exec(ctx); err != nil {
		return fmt.Errorf("close assigned schedule snapshots: %w", err)
	}

	updateStaffQuery := db.NewUpdate().
		TableExpr(`users.staff`).
		Set("rotation_anchor_date = ?", model.RotationAnchorDate).
		Where("id IN (?)", bun.List(staffIDs)).
		Where("work_time_model_id = ?", modelID)
	if tenantID > 0 {
		updateStaffQuery = updateStaffQuery.Where("tenant_id = ?", tenantID)
	}
	if _, err := updateStaffQuery.Exec(ctx); err != nil {
		return fmt.Errorf("update assigned staff rotation anchor: %w", err)
	}

	if len(model.Entries) == 0 {
		return nil
	}
	rows := make([]*config.StaffWorkSchedule, 0, len(staffIDs)*len(model.Entries))
	for _, staffID := range staffIDs {
		for _, entry := range model.Entries {
			// Stamp the model's anchor onto the new version (#1842). The
			// staff-level anchor was just overwritten above, so without this
			// the fresh rows would inherit a moving anchor — the very thing
			// that let a template edit re-parity closed weeks.
			versionAnchor := model.RotationAnchorDate
			row := &config.StaffWorkSchedule{
				StaffID:            staffID,
				WeekIndex:          entry.WeekIndex,
				RotationLength:     model.RotationLength,
				DayOfWeek:          entry.DayOfWeek,
				TargetMinutes:      entry.TargetMinutes,
				StartTime:          entry.StartTime,
				RotationAnchorDate: &versionAnchor,
				ValidFrom:          today,
				ValidUntil:         nil,
				CreatedAt:          now,
				UpdatedAt:          now,
			}
			if tenantID > 0 {
				row.SetTenantID(tenantID)
			}
			rows = append(rows, row)
		}
	}
	if _, err := db.NewInsert().Model(&rows).ModelTableExpr(tableStaffWorkSchedules).Exec(ctx); err != nil {
		return fmt.Errorf("insert assigned schedule snapshots: %w", err)
	}
	return nil
}

// Delete removes a template; entries cascade via the FK.
func (r *WorkTimeModelRepository) Delete(ctx context.Context, id int64) error {
	db := r.runtime.DB(ctx)
	assignedQuery := db.NewSelect().
		TableExpr(`users.staff AS "staff"`).
		Where(`"staff".work_time_model_id = ?`, id).
		Where(`"staff".deleted_at IS NULL`)
	if tenantID := r.runtime.TenantID(ctx); tenantID > 0 {
		assignedQuery = assignedQuery.Where(`"staff".tenant_id = ?`, tenantID)
	}
	assignedCount, err := assignedQuery.Count(ctx)
	if err != nil {
		return fmt.Errorf("check assigned staff: %w", err)
	}
	if assignedCount > 0 {
		return fmt.Errorf("work time model is assigned to staff")
	}

	query := db.NewDelete().
		Model((*config.WorkTimeModel)(nil)).
		ModelTableExpr(tableWorkTimeModels).
		Where("id = ?", id)
	if tenantID := r.runtime.TenantID(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}
	res, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func validateEntriesForRotation(entries []*config.WorkTimeModelEntry, rotation int) error {
	for _, e := range entries {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("invalid entry (week=%d, day=%d): %w", e.WeekIndex, e.DayOfWeek, err)
		}
		if e.WeekIndex >= rotation {
			return errors.New("entry week_index outside rotation_length")
		}
	}
	return nil
}
