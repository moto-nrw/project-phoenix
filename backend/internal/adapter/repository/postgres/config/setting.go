package config

import (
	"context"
	"fmt"
	"strings"

	repoBase "github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// Table constants
const (
	tableConfigSettings      = "config.settings"
	tableConfigSettingsAlias = `config.settings AS "setting"`
)

// SettingRepository implements config.SettingRepository interface
type SettingRepository struct {
	*repoBase.Repository[*config.Setting]
	db *bun.DB
}

// NewSettingRepository creates a new SettingRepository
func NewSettingRepository(db *bun.DB) config.SettingRepository {
	return &SettingRepository{
		Repository: repoBase.NewRepository[*config.Setting](db, tableConfigSettings, "Setting"),
		db:         db,
	}
}

// FindByKey retrieves a setting by its key
func (r *SettingRepository) FindByKey(ctx context.Context, key string) (*config.Setting, error) {
	// Normalize key to follow the project convention
	key = strings.ToLower(strings.ReplaceAll(key, " ", "_"))

	setting := new(config.Setting)
	err := r.db.NewSelect().
		Model(setting).
		ModelTableExpr(tableConfigSettingsAlias).
		Where("key = ?", key).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by key",
			Err: err,
		}
	}

	return setting, nil
}

// FindByCategory retrieves settings by their category
func (r *SettingRepository) FindByCategory(ctx context.Context, category string) ([]*config.Setting, error) {
	// Normalize category to follow the project convention
	category = strings.ToLower(category)

	var settings []*config.Setting
	err := r.db.NewSelect().
		Model(&settings).
		ModelTableExpr(tableConfigSettingsAlias).
		Where("category = ?", category).
		Order("key ASC").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by category",
			Err: err,
		}
	}

	return settings, nil
}

// FindByKeyAndCategory retrieves a setting by its key and category
func (r *SettingRepository) FindByKeyAndCategory(ctx context.Context, key string, category string) (*config.Setting, error) {
	// Normalize key and category to follow the project convention
	key = strings.ToLower(strings.ReplaceAll(key, " ", "_"))
	category = strings.ToLower(category)

	setting := new(config.Setting)
	err := r.db.NewSelect().
		Model(setting).
		ModelTableExpr(tableConfigSettingsAlias).
		Where("key = ? AND category = ?", key, category).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by key and category",
			Err: err,
		}
	}

	return setting, nil
}

// UpdateValue updates the value of a setting identified by its key
func (r *SettingRepository) UpdateValue(ctx context.Context, key string, value string) error {
	// Normalize key to follow the project convention
	key = strings.ToLower(strings.ReplaceAll(key, " ", "_"))

	_, err := r.db.NewUpdate().
		Model((*config.Setting)(nil)).
		ModelTableExpr(tableConfigSettingsAlias).
		Set("value = ?", value).
		Where("key = ?", key).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update value",
			Err: err,
		}
	}

	return nil
}

// GetValue retrieves the value of a setting by its key
func (r *SettingRepository) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.FindByKey(ctx, key)
	if err != nil {
		return "", &modelBase.DatabaseError{
			Op:  "get value",
			Err: err,
		}
	}

	return setting.Value, nil
}

// GetBoolValue retrieves the boolean value of a setting by its key
func (r *SettingRepository) GetBoolValue(ctx context.Context, key string) (bool, error) {
	setting, err := r.FindByKey(ctx, key)
	if err != nil {
		return false, &modelBase.DatabaseError{
			Op:  "get bool value",
			Err: err,
		}
	}

	return setting.GetBoolValue(), nil
}

// GetFullKey constructs the full key for a setting using its category and key
func (r *SettingRepository) GetFullKey(_ context.Context, category string, key string) (string, error) {
	// Normalize key and category to follow the project convention
	key = strings.ToLower(strings.ReplaceAll(key, " ", "_"))
	category = strings.ToLower(category)

	return category + "." + key, nil
}

// Create overrides the base Create method to handle validation
func (r *SettingRepository) Create(ctx context.Context, setting *config.Setting) error {
	if setting == nil {
		return fmt.Errorf("setting cannot be nil")
	}

	// Validate setting
	if err := setting.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, setting)
}

// Update overrides the base Update method to handle validation
func (r *SettingRepository) Update(ctx context.Context, setting *config.Setting) error {
	if setting == nil {
		return fmt.Errorf("setting cannot be nil")
	}

	// Validate setting
	if err := setting.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, setting)
}

// List retrieves settings matching the provided filters
func (r *SettingRepository) List(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error) {
	var settings []*config.Setting
	query := r.db.NewSelect().Model(&settings).ModelTableExpr(tableConfigSettingsAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = applySettingFilter(query, field, value)
		}
	}

	// Default ordering
	query = query.Order("category").Order("key")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return settings, nil
}

// applySettingFilter applies a single filter to the query based on field name
func applySettingFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "key_like":
		return applyLikeFilter(query, "key", value)
	case "category_like":
		return applyLikeFilter(query, "category", value)
	case "value_like":
		return applyLikeFilter(query, "value", value)
	case "requires_restart", "requires_db_reset":
		return applyBooleanFilter(query, field, value)
	default:
		return applyDefaultFilter(query, field, value)
	}
}

// applyLikeFilter applies a LIKE filter if value is a string
func applyLikeFilter(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where(column+" ILIKE ?", "%"+strValue+"%")
	}
	return query
}

// applyBooleanFilter applies a boolean filter
func applyBooleanFilter(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	return query.Where(column+" = ?", value)
}

// applyDefaultFilter applies a default exact match filter
func applyDefaultFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	return query.Where("? = ?", bun.Ident(field), value)
}
