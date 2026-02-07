package config

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// SettingDefinitionRepository implements config.SettingDefinitionRepository
type SettingDefinitionRepository struct {
	db *bun.DB
}

// NewSettingDefinitionRepository creates a new setting definition repository
func NewSettingDefinitionRepository(db *bun.DB) *SettingDefinitionRepository {
	return &SettingDefinitionRepository{db: db}
}

// Create inserts a new setting definition
func (r *SettingDefinitionRepository) Create(ctx context.Context, def *config.SettingDefinition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(def).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Returning("*").
		Exec(ctx)
	return err
}

// Update modifies an existing setting definition
func (r *SettingDefinitionRepository) Update(ctx context.Context, def *config.SettingDefinition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewUpdate().
		Model(def).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".id = ?`, def.ID).
		Where(`"setting_definition".deleted_at IS NULL`).
		Returning("*").
		Exec(ctx)
	return err
}

// FindByID retrieves a definition by ID (excludes soft-deleted)
func (r *SettingDefinitionRepository) FindByID(ctx context.Context, id int64) (*config.SettingDefinition, error) {
	var def config.SettingDefinition
	err := r.db.NewSelect().
		Model(&def).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".id = ?`, id).
		Where(`"setting_definition".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

// FindByKey retrieves a definition by key (excludes soft-deleted)
func (r *SettingDefinitionRepository) FindByKey(ctx context.Context, key string) (*config.SettingDefinition, error) {
	var def config.SettingDefinition
	err := r.db.NewSelect().
		Model(&def).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".key = ?`, key).
		Where(`"setting_definition".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &def, nil
}

// FindByKeys retrieves definitions by keys (excludes soft-deleted)
func (r *SettingDefinitionRepository) FindByKeys(ctx context.Context, keys []string) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".key IN (?)`, bun.In(keys)).
		Where(`"setting_definition".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return defs, nil
}

// FindByTab retrieves definitions for a specific tab
func (r *SettingDefinitionRepository) FindByTab(ctx context.Context, tab string) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".tab = ?`, tab).
		Where(`"setting_definition".deleted_at IS NULL`).
		Order("display_order ASC", "category ASC", "key ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return defs, nil
}

// FindByCategory retrieves definitions for a specific category
func (r *SettingDefinitionRepository) FindByCategory(ctx context.Context, category string) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".category = ?`, category).
		Where(`"setting_definition".deleted_at IS NULL`).
		Order("display_order ASC", "key ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return defs, nil
}

// FindAll retrieves all active definitions
func (r *SettingDefinitionRepository) FindAll(ctx context.Context) ([]*config.SettingDefinition, error) {
	var defs []*config.SettingDefinition
	err := r.db.NewSelect().
		Model(&defs).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Where(`"setting_definition".deleted_at IS NULL`).
		Order("tab ASC", "category ASC", "display_order ASC", "key ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return defs, nil
}

// SoftDelete marks a definition as deleted
func (r *SettingDefinitionRepository) SoftDelete(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*config.SettingDefinition)(nil)).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Set("deleted_at = ?", now).
		Where(`"setting_definition".id = ?`, id).
		Where(`"setting_definition".deleted_at IS NULL`).
		Exec(ctx)
	return err
}

// Restore unmarks a soft-deleted definition
func (r *SettingDefinitionRepository) Restore(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*config.SettingDefinition)(nil)).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		Set("deleted_at = NULL").
		Where(`"setting_definition".id = ?`, id).
		Where(`"setting_definition".deleted_at IS NOT NULL`).
		Exec(ctx)
	return err
}

// PurgeDeletedOlderThan permanently removes definitions deleted before the given days
func (r *SettingDefinitionRepository) PurgeDeletedOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result, err := r.db.NewDelete().
		Model((*config.SettingDefinition)(nil)).
		ModelTableExpr(`config.setting_definitions`).
		Where("deleted_at IS NOT NULL").
		Where("deleted_at < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Upsert creates or updates a definition by key
func (r *SettingDefinitionRepository) Upsert(ctx context.Context, def *config.SettingDefinition) error {
	if err := def.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(def).
		ModelTableExpr(`config.setting_definitions AS "setting_definition"`).
		On("CONFLICT (key) WHERE deleted_at IS NULL DO UPDATE").
		Set("value_type = EXCLUDED.value_type").
		Set("default_value = EXCLUDED.default_value").
		Set("category = EXCLUDED.category").
		Set("tab = EXCLUDED.tab").
		Set("display_order = EXCLUDED.display_order").
		Set("label = EXCLUDED.label").
		Set("description = EXCLUDED.description").
		Set("allowed_scopes = EXCLUDED.allowed_scopes").
		Set("view_permission = EXCLUDED.view_permission").
		Set("edit_permission = EXCLUDED.edit_permission").
		Set("validation_schema = EXCLUDED.validation_schema").
		Set("enum_values = EXCLUDED.enum_values").
		Set("enum_options = EXCLUDED.enum_options").
		Set("object_ref_type = EXCLUDED.object_ref_type").
		Set("object_ref_filter = EXCLUDED.object_ref_filter").
		Set("requires_restart = EXCLUDED.requires_restart").
		Set("is_sensitive = EXCLUDED.is_sensitive").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)
	return err
}
