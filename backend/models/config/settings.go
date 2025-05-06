package config

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Setting represents a configuration setting in the system
type Setting struct {
	base.Model
	Key             string `bun:"key,notnull,unique" json:"key"`
	Value           string `bun:"value,notnull" json:"value"`
	Category        string `bun:"category,notnull" json:"category"`
	Description     string `bun:"description" json:"description,omitempty"`
	RequiresRestart bool   `bun:"requires_restart,notnull,default:false" json:"requires_restart"`
	RequiresDBReset bool   `bun:"requires_db_reset,notnull,default:false" json:"requires_db_reset"`
}

// TableName returns the table name for the Setting model
func (s *Setting) TableName() string {
	return "config.settings"
}

// GetID returns the setting ID
func (s *Setting) GetID() interface{} {
	return s.ID
}

// GetCreatedAt returns the creation timestamp
func (s *Setting) GetCreatedAt() time.Time {
	return s.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (s *Setting) GetUpdatedAt() time.Time {
	return s.UpdatedAt
}

// Validate validates the setting fields
func (s *Setting) Validate() error {
	if strings.TrimSpace(s.Key) == "" {
		return errors.New("key is required")
	}

	if strings.TrimSpace(s.Value) == "" {
		return errors.New("value is required")
	}

	if strings.TrimSpace(s.Category) == "" {
		return errors.New("category is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (s *Setting) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := s.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	s.Key = strings.TrimSpace(s.Key)
	s.Value = strings.TrimSpace(s.Value)
	s.Category = strings.TrimSpace(s.Category)
	s.Description = strings.TrimSpace(s.Description)

	return nil
}

// SettingRepository defines operations for working with settings
type SettingRepository interface {
	base.Repository[*Setting]
	FindByKey(ctx context.Context, key string) (*Setting, error)
	FindByCategory(ctx context.Context, category string) ([]*Setting, error)
	FindByKeyPrefix(ctx context.Context, prefix string) ([]*Setting, error)
	UpdateValue(ctx context.Context, key string, value string) error
	GetValue(ctx context.Context, key string) (string, error)
	GetValueWithDefault(ctx context.Context, key string, defaultValue string) (string, error)
	FindRequiringRestart(ctx context.Context) ([]*Setting, error)
	FindRequiringDBReset(ctx context.Context) ([]*Setting, error)
}

// DefaultSettingRepository is the default implementation of SettingRepository
type DefaultSettingRepository struct {
	db *bun.DB
}

// NewSettingRepository creates a new setting repository
func NewSettingRepository(db *bun.DB) SettingRepository {
	return &DefaultSettingRepository{db: db}
}

// Create inserts a new setting into the database
func (r *DefaultSettingRepository) Create(ctx context.Context, setting *Setting) error {
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
func (r *DefaultSettingRepository) FindByID(ctx context.Context, id interface{}) (*Setting, error) {
	setting := new(Setting)
	err := r.db.NewSelect().Model(setting).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return setting, nil
}

// FindByKey retrieves a setting by its key
func (r *DefaultSettingRepository) FindByKey(ctx context.Context, key string) (*Setting, error) {
	setting := new(Setting)
	err := r.db.NewSelect().Model(setting).Where("key = ?", key).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_key", Err: err}
	}
	return setting, nil
}

// FindByCategory retrieves settings by category
func (r *DefaultSettingRepository) FindByCategory(ctx context.Context, category string) ([]*Setting, error) {
	var settings []*Setting
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
func (r *DefaultSettingRepository) FindByKeyPrefix(ctx context.Context, prefix string) ([]*Setting, error) {
	var settings []*Setting
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
func (r *DefaultSettingRepository) UpdateValue(ctx context.Context, key string, value string) error {
	_, err := r.db.NewUpdate().
		Model((*Setting)(nil)).
		Set("value = ?", value).
		Where("key = ?", key).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_value", Err: err}
	}
	return nil
}

// GetValue retrieves the value of a setting by key
func (r *DefaultSettingRepository) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.FindByKey(ctx, key)
	if err != nil {
		return "", &base.DatabaseError{Op: "get_value", Err: err}
	}
	return setting.Value, nil
}

// GetValueWithDefault retrieves the value of a setting by key, or returns the default value if not found
func (r *DefaultSettingRepository) GetValueWithDefault(ctx context.Context, key string, defaultValue string) (string, error) {
	setting, err := r.FindByKey(ctx, key)
	if err != nil {
		// If the setting doesn't exist, return the default value
		return defaultValue, nil
	}
	return setting.Value, nil
}

// FindRequiringRestart retrieves all settings that require a restart
func (r *DefaultSettingRepository) FindRequiringRestart(ctx context.Context) ([]*Setting, error) {
	var settings []*Setting
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
func (r *DefaultSettingRepository) FindRequiringDBReset(ctx context.Context) ([]*Setting, error) {
	var settings []*Setting
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
func (r *DefaultSettingRepository) Update(ctx context.Context, setting *Setting) error {
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
func (r *DefaultSettingRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Setting)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves settings matching the filters
func (r *DefaultSettingRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Setting, error) {
	var settings []*Setting
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
