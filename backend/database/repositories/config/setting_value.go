package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// SettingValueRepository implements config.SettingValueRepository
type SettingValueRepository struct {
	db *bun.DB
}

// NewSettingValueRepository creates a new setting value repository
func NewSettingValueRepository(db *bun.DB) *SettingValueRepository {
	return &SettingValueRepository{db: db}
}

// Create inserts a new setting value
func (r *SettingValueRepository) Create(ctx context.Context, value *config.SettingValue) error {
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(value).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Returning("*").
		Exec(ctx)
	return err
}

// Update modifies an existing setting value
func (r *SettingValueRepository) Update(ctx context.Context, value *config.SettingValue) error {
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewUpdate().
		Model(value).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Where(`"setting_value".id = ?`, value.ID).
		Where(`"setting_value".deleted_at IS NULL`).
		Returning("*").
		Exec(ctx)
	return err
}

// FindByID retrieves a value by ID (excludes soft-deleted)
func (r *SettingValueRepository) FindByID(ctx context.Context, id int64) (*config.SettingValue, error) {
	var value config.SettingValue
	err := r.db.NewSelect().
		Model(&value).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Where(`"setting_value".id = ?`, id).
		Where(`"setting_value".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// FindByDefinitionAndScope retrieves a value for a specific definition and scope
func (r *SettingValueRepository) FindByDefinitionAndScope(ctx context.Context, defID int64, scopeType config.Scope, scopeID *int64) (*config.SettingValue, error) {
	var value config.SettingValue
	query := r.db.NewSelect().
		Model(&value).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Where(`"setting_value".definition_id = ?`, defID).
		Where(`"setting_value".scope_type = ?`, scopeType).
		Where(`"setting_value".deleted_at IS NULL`)

	if scopeID == nil {
		query = query.Where(`"setting_value".scope_id IS NULL`)
	} else {
		query = query.Where(`"setting_value".scope_id = ?`, *scopeID)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// FindByDefinitionID retrieves all values for a definition (excludes soft-deleted)
func (r *SettingValueRepository) FindByDefinitionID(ctx context.Context, defID int64) ([]*config.SettingValue, error) {
	var values []*config.SettingValue
	err := r.db.NewSelect().
		Model(&values).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Where(`"setting_value".definition_id = ?`, defID).
		Where(`"setting_value".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// FindEffectiveValue returns the value at the highest-priority scope
func (r *SettingValueRepository) FindEffectiveValue(ctx context.Context, defID int64, scopeCtx *config.ScopeContext) (*config.SettingValue, config.Scope, error) {
	// Try scopes in priority order: device > user > system
	scopes := config.AllScopes() // Returns [device, user, system]

	for _, scope := range scopes {
		var scopeID *int64

		switch scope {
		case config.ScopeDevice:
			if scopeCtx == nil || scopeCtx.DeviceID == nil {
				continue
			}
			scopeID = scopeCtx.DeviceID
		case config.ScopeUser:
			if scopeCtx == nil || scopeCtx.AccountID == nil {
				continue
			}
			scopeID = scopeCtx.AccountID
		case config.ScopeSystem:
			// System scope has no scopeID
			scopeID = nil
		}

		value, err := r.FindByDefinitionAndScope(ctx, defID, scope, scopeID)
		if err == nil && value != nil {
			return value, scope, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, "", err
		}
	}

	return nil, "", sql.ErrNoRows
}

// FindByScopeType retrieves all values for a scope type
func (r *SettingValueRepository) FindByScopeType(ctx context.Context, scopeType config.Scope) ([]*config.SettingValue, error) {
	var values []*config.SettingValue
	err := r.db.NewSelect().
		Model(&values).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Where(`"setting_value".scope_type = ?`, scopeType).
		Where(`"setting_value".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// FindByScopeEntity retrieves all values for a specific scope entity
func (r *SettingValueRepository) FindByScopeEntity(ctx context.Context, scopeType config.Scope, scopeID int64) ([]*config.SettingValue, error) {
	var values []*config.SettingValue
	err := r.db.NewSelect().
		Model(&values).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Where(`"setting_value".scope_type = ?`, scopeType).
		Where(`"setting_value".scope_id = ?`, scopeID).
		Where(`"setting_value".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// SoftDelete marks a value as deleted
func (r *SettingValueRepository) SoftDelete(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Set("deleted_at = ?", now).
		Where(`"setting_value".id = ?`, id).
		Where(`"setting_value".deleted_at IS NULL`).
		Exec(ctx)
	return err
}

// SoftDeleteByScope soft deletes a value by definition and scope
func (r *SettingValueRepository) SoftDeleteByScope(ctx context.Context, defID int64, scopeType config.Scope, scopeID *int64) error {
	now := time.Now()
	query := r.db.NewUpdate().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Set("deleted_at = ?", now).
		Where(`"setting_value".definition_id = ?`, defID).
		Where(`"setting_value".scope_type = ?`, scopeType).
		Where(`"setting_value".deleted_at IS NULL`)

	if scopeID == nil {
		query = query.Where(`"setting_value".scope_id IS NULL`)
	} else {
		query = query.Where(`"setting_value".scope_id = ?`, *scopeID)
	}

	_, err := query.Exec(ctx)
	return err
}

// Restore unmarks a soft-deleted value
func (r *SettingValueRepository) Restore(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		Set("deleted_at = NULL").
		Where(`"setting_value".id = ?`, id).
		Where(`"setting_value".deleted_at IS NOT NULL`).
		Exec(ctx)
	return err
}

// PurgeDeletedOlderThan permanently removes values deleted before the given days
func (r *SettingValueRepository) PurgeDeletedOlderThan(ctx context.Context, days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result, err := r.db.NewDelete().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(`config.setting_values`).
		Where("deleted_at IS NOT NULL").
		Where("deleted_at < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Upsert creates or updates a value by definition and scope
func (r *SettingValueRepository) Upsert(ctx context.Context, value *config.SettingValue) error {
	if err := value.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Use COALESCE for scope_id to handle NULL values in unique constraint
	_, err := r.db.NewInsert().
		Model(value).
		ModelTableExpr(`config.setting_values AS "setting_value"`).
		On("CONFLICT (definition_id, scope_type, COALESCE(scope_id, -1)) WHERE deleted_at IS NULL DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)
	return err
}

// DeleteByScopeEntity deletes all values for a scope entity
func (r *SettingValueRepository) DeleteByScopeEntity(ctx context.Context, scopeType config.Scope, scopeID int64) error {
	_, err := r.db.NewDelete().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(`config.setting_values`).
		Where("scope_type = ?", scopeType).
		Where("scope_id = ?", scopeID).
		Exec(ctx)
	return err
}
