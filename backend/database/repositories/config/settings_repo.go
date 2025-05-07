package config

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// SettingRepository implements config.SettingRepository
type SettingRepository struct {
	db *bun.DB
}

// NewSettingRepository creates a new setting repository
func NewSettingRepository(db *bun.DB) config.SettingRepository {
	return &SettingRepository{db: db}
}

// Create inserts a new setting into the database
func (r *SettingRepository) Create(ctx context.Context, setting *config.Setting) error {
	if err := setting.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(setting).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a setting by its ID
func (r *SettingRepository) FindByID(ctx context.Context, id interface{}) (*config.Setting, error) {
	setting := new(config.Setting)
	err := r.db.NewSelect().Model(setting).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return setting, nil
}

// FindByKey retrieves a setting by its key
func (r *SettingRepository) FindByKey(ctx context.Context, key string) (*config.Setting, error) {
	setting := new(config.Setting)
	err := r.db.NewSelect().Model(setting).Where("key = ?", key).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_key", Err: err}
	}
	return setting, nil
}

// FindByCategory retrieves settings by category
func (r *SettingRepository) FindByCategory(ctx context.Context, category string) ([]*config.Setting, error) {
	var settings []*config.Setting
	err := r.db.NewSelect().
		Model(&settings).
		Where("category = ?", category).
		Order("key ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_category", Err: err}
	}
	return settings, nil
}

// FindByKeyPrefix retrieves settings by key prefix
func (r *SettingRepository) FindByKeyPrefix(ctx context.Context, prefix string) ([]*config.Setting, error) {
	var settings []*config.Setting
	err := r.db.NewSelect().
		Model(&settings).
		Where("key LIKE ?", prefix+"%").
		Order("key ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_key_prefix", Err: err}
	}
	return settings, nil
}

// UpdateValue updates the value of a setting by key
func (r *SettingRepository) UpdateValue(ctx context.Context, key string, value string) error {
	_, err := r.db.NewUpdate().
		Model((*config.Setting)(nil)).
		Set("value = ?", value).
		Where("key = ?", key).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_value", Err: err}
	}
	return nil
}

// GetValue retrieves the value of a setting by key
func (r *SettingRepository) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.FindByKey(ctx, key)
	if err != nil {
		return "", &base.DatabaseError{Op: "get_value", Err: err}
	}
	return setting.Value, nil
}

// GetValueWithDefault retrieves the value of a setting by key, or returns the default value if not found
func (r *SettingRepository) GetValueWithDefault(ctx context.Context, key string, defaultValue string) (string, error) {
	setting, err := r.FindByKey(ctx, key)
	if err != nil {
		// If the setting doesn't exist, return the default value
		return defaultValue, nil
	}
	return setting.Value, nil
}

// FindRequiringRestart retrieves all settings that require a restart
func (r *SettingRepository) FindRequiringRestart(ctx context.Context) ([]*config.Setting, error) {
	var settings []*config.Setting
	err := r.db.NewSelect().
		Model(&settings).
		Where("requires_restart = ?", true).
		Order("key ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_requiring_restart", Err: err}
	}
	return settings, nil
}

// FindRequiringDBReset retrieves all settings that require a database reset
func (r *SettingRepository) FindRequiringDBReset(ctx context.Context) ([]*config.Setting, error) {
	var settings []*config.Setting
	err := r.db.NewSelect().
		Model(&settings).
		Where("requires_db_reset = ?", true).
		Order("key ASC").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_requiring_db_reset", Err: err}
	}
	return settings, nil
}

// Update updates an existing setting
func (r *SettingRepository) Update(ctx context.Context, setting *config.Setting) error {
	if err := setting.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(setting).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a setting
func (r *SettingRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*config.Setting)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves settings matching the filters
func (r *SettingRepository) List(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error) {
	var settings []*config.Setting
	query := r.db.NewSelect().Model(&settings)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return settings, nil
}
