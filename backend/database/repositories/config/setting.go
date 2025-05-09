package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// SettingRepository implements config.SettingRepository interface
type SettingRepository struct {
	*base.Repository[*config.Setting]
	db *bun.DB
}

// NewSettingRepository creates a new SettingRepository
func NewSettingRepository(db *bun.DB) config.SettingRepository {
	return &SettingRepository{
		Repository: base.NewRepository[*config.Setting](db, "config.settings", "Setting"),
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
func (r *SettingRepository) GetFullKey(ctx context.Context, category string, key string) (string, error) {
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
	query := r.db.NewSelect().Model(&settings)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "key_like":
				if strValue, ok := value.(string); ok {
					query = query.Where("key ILIKE ?", "%"+strValue+"%")
				}
			case "category_like":
				if strValue, ok := value.(string); ok {
					query = query.Where("category ILIKE ?", "%"+strValue+"%")
				}
			case "value_like":
				if strValue, ok := value.(string); ok {
					query = query.Where("value ILIKE ?", "%"+strValue+"%")
				}
			case "requires_restart":
				query = query.Where("requires_restart = ?", value)
			case "requires_db_reset":
				query = query.Where("requires_db_reset = ?", value)
			default:
				// Default to exact match for other fields
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}
	}

	// Default ordering
	query = query.Order("category, key")

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return settings, nil
}
