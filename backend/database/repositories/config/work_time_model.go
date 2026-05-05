package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableWorkTimeModels       = "config.work_time_models"
	tableWorkTimeModelEntries = "config.work_time_model_entries"
)

// WorkTimeModelRepository implements config.WorkTimeModelRepository on top of bun.
type WorkTimeModelRepository struct {
	db *bun.DB
}

// NewWorkTimeModelRepository wires the bun-backed implementation.
func NewWorkTimeModelRepository(db *bun.DB) config.WorkTimeModelRepository {
	return &WorkTimeModelRepository{db: db}
}

// List returns every template visible to the active tenant, eagerly loading entries.
func (r *WorkTimeModelRepository) List(ctx context.Context) ([]*config.WorkTimeModel, error) {
	var models []*config.WorkTimeModel
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&models).
		ModelTableExpr(tableWorkTimeModels + ` AS "work_time_model"`).
		Relation("Entries").
		OrderExpr(`"work_time_model".name ASC`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"work_time_model".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list work-time models: %w", err)
	}
	return models, nil
}

// FindByID resolves a single template with its entries; returns sql.ErrNoRows when missing.
func (r *WorkTimeModelRepository) FindByID(ctx context.Context, id int64) (*config.WorkTimeModel, error) {
	model := new(config.WorkTimeModel)
	query := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(model).
		ModelTableExpr(tableWorkTimeModels+` AS "work_time_model"`).
		Relation("Entries").
		Where(`"work_time_model".id = ?`, id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"work_time_model".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, err
	}
	return model, nil
}

// Create inserts the template and its entries inside one transaction.
func (r *WorkTimeModelRepository) Create(ctx context.Context, model *config.WorkTimeModel, entries []*config.WorkTimeModelEntry) error {
	db := repoBase.GetDB(ctx, r.db)
	tenantID := tenant.FromContext(ctx)
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
	db := repoBase.GetDB(ctx, r.db)
	tenantID := tenant.FromContext(ctx)
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
	if _, err := updateQuery.Exec(ctx); err != nil {
		return fmt.Errorf("update model: %w", err)
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

// Delete removes a template; entries cascade via the FK.
func (r *WorkTimeModelRepository) Delete(ctx context.Context, id int64) error {
	db := repoBase.GetDB(ctx, r.db)
	query := db.NewDelete().
		Model((*config.WorkTimeModel)(nil)).
		ModelTableExpr(tableWorkTimeModels).
		Where("id = ?", id)
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
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
