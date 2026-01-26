package config

import (
	"context"
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// Table and query constants for setting values
const (
	tableSettingValues      = "config.setting_values"
	tableSettingValuesAlias = `config.setting_values AS "setting_value"`

	whereScopeType = "scope_type = ?"
	whereScopeID   = "scope_id = ?"
)

// SettingValueRepository implements config.SettingValueRepository
type SettingValueRepository struct {
	db *bun.DB
}

// NewSettingValueRepository creates a new SettingValueRepository
func NewSettingValueRepository(db *bun.DB) config.SettingValueRepository {
	return &SettingValueRepository{db: db}
}

// Create inserts a new setting value
func (r *SettingValueRepository) Create(ctx context.Context, value *config.SettingValue) error {
	if err := value.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(value).
		ModelTableExpr(tableSettingValues).
		Returning("*").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create setting value",
			Err: err,
		}
	}

	return nil
}

// FindByID retrieves a setting value by its ID
func (r *SettingValueRepository) FindByID(ctx context.Context, id int64) (*config.SettingValue, error) {
	value := new(config.SettingValue)
	err := r.db.NewSelect().
		Model(value).
		ModelTableExpr(tableSettingValuesAlias).
		Where(whereID, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find value by id",
			Err: err,
		}
	}

	return value, nil
}

// Update modifies an existing setting value
func (r *SettingValueRepository) Update(ctx context.Context, value *config.SettingValue) error {
	if err := value.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().
		Model(value).
		ModelTableExpr(tableSettingValues).
		Where(whereID, value.ID).
		Returning("*").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update setting value",
			Err: err,
		}
	}

	return nil
}

// Delete removes a setting value
func (r *SettingValueRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.NewDelete().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(tableSettingValues).
		Where(whereID, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete setting value",
			Err: err,
		}
	}

	return nil
}

// FindByDefinitionAndScope retrieves a setting value for a specific definition and scope
func (r *SettingValueRepository) FindByDefinitionAndScope(
	ctx context.Context,
	definitionID int64,
	scopeType string,
	scopeID *int64,
) (*config.SettingValue, error) {
	value := new(config.SettingValue)
	query := r.db.NewSelect().
		Model(value).
		ModelTableExpr(tableSettingValuesAlias).
		Where("definition_id = ?", definitionID).
		Where(whereScopeType, scopeType)

	if scopeID == nil {
		query = query.Where("scope_id IS NULL")
	} else {
		query = query.Where(whereScopeID, *scopeID)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find value by definition and scope",
			Err: err,
		}
	}

	return value, nil
}

// FindAllForScope retrieves all setting values for a specific scope
func (r *SettingValueRepository) FindAllForScope(
	ctx context.Context,
	scopeType string,
	scopeID *int64,
) ([]*config.SettingValue, error) {
	var values []*config.SettingValue
	query := r.db.NewSelect().
		Model(&values).
		ModelTableExpr(tableSettingValuesAlias).
		Where(whereScopeType, scopeType)

	if scopeID == nil {
		query = query.Where("scope_id IS NULL")
	} else {
		query = query.Where(whereScopeID, *scopeID)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find all values for scope",
			Err: err,
		}
	}

	return values, nil
}

// FindByDefinition retrieves all values for a specific definition
func (r *SettingValueRepository) FindByDefinition(ctx context.Context, definitionID int64) ([]*config.SettingValue, error) {
	var values []*config.SettingValue
	err := r.db.NewSelect().
		Model(&values).
		ModelTableExpr(tableSettingValuesAlias).
		Where("definition_id = ?", definitionID).
		Order("scope_type ASC", "scope_id ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find values by definition",
			Err: err,
		}
	}

	return values, nil
}

// Upsert inserts or updates a setting value
func (r *SettingValueRepository) Upsert(ctx context.Context, value *config.SettingValue) error {
	if err := value.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().
		Model(value).
		ModelTableExpr(tableSettingValues).
		On("CONFLICT (definition_id, scope_type, scope_id) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("set_by = EXCLUDED.set_by").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "upsert setting value",
			Err: err,
		}
	}

	return nil
}

// DeleteByScope removes all setting values for a specific scope
func (r *SettingValueRepository) DeleteByScope(ctx context.Context, scopeType string, scopeID int64) (int, error) {
	result, err := r.db.NewDelete().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(tableSettingValues).
		Where(whereScopeType, scopeType).
		Where(whereScopeID, scopeID).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete values by scope",
			Err: err,
		}
	}

	count, _ := result.RowsAffected()
	return int(count), nil
}
