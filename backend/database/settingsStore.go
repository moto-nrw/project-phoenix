package database

import (
	"context"
	"github.com/moto-nrw/project-phoenix/models"

	"github.com/uptrace/bun"
)

// SettingsStore defines the interface for the settings store
type SettingsStore interface {
	// Create creates a new setting
	Create(ctx context.Context, setting *models.Setting) error
	// Update updates an existing setting
	Update(ctx context.Context, id int64, setting *models.Setting) error
	// UpdateByKey updates a setting by its key
	UpdateByKey(ctx context.Context, key string, value string) error
	// Get retrieves a setting by ID
	Get(ctx context.Context, id int64) (*models.Setting, error)
	// GetByKey retrieves a setting by its key
	GetByKey(ctx context.Context, key string) (*models.Setting, error)
	// GetByCategory retrieves all settings by category
	GetByCategory(ctx context.Context, category string) ([]*models.Setting, error)
	// List retrieves all settings
	List(ctx context.Context) ([]*models.Setting, error)
	// Delete deletes a setting
	Delete(ctx context.Context, id int64) error
}

// BunSettingsStore implements SettingsStore using Bun ORM
type BunSettingsStore struct {
	db *bun.DB
}

// This function is defined in postgres.go
// func NewSettingsStore(db *bun.DB) *BunSettingsStore {
// 	return &BunSettingsStore{db: db}
// }

// Create creates a new setting
func (s *BunSettingsStore) Create(ctx context.Context, setting *models.Setting) error {
	_, err := s.db.NewInsert().
		Model(setting).
		Exec(ctx)

	return err
}

// Update updates an existing setting
func (s *BunSettingsStore) Update(ctx context.Context, id int64, setting *models.Setting) error {
	_, err := s.db.NewUpdate().
		Model(setting).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// UpdateByKey updates a setting by its key
func (s *BunSettingsStore) UpdateByKey(ctx context.Context, key string, value string) error {
	_, err := s.db.NewUpdate().
		Table("settings").
		Set("value = ?", value).
		Set("modified_at = NOW()").
		Where("key = ?", key).
		Exec(ctx)

	return err
}

// Get retrieves a setting by ID
func (s *BunSettingsStore) Get(ctx context.Context, id int64) (*models.Setting, error) {
	setting := new(models.Setting)
	err := s.db.NewSelect().
		Model(setting).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return setting, nil
}

// GetByKey retrieves a setting by its key
func (s *BunSettingsStore) GetByKey(ctx context.Context, key string) (*models.Setting, error) {
	setting := new(models.Setting)
	err := s.db.NewSelect().
		Model(setting).
		Where("key = ?", key).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return setting, nil
}

// GetByCategory retrieves all settings by category
func (s *BunSettingsStore) GetByCategory(ctx context.Context, category string) ([]*models.Setting, error) {
	var settings []*models.Setting
	err := s.db.NewSelect().
		Model(&settings).
		Where("category = ?", category).
		OrderExpr("key ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return settings, nil
}

// List retrieves all settings
func (s *BunSettingsStore) List(ctx context.Context) ([]*models.Setting, error) {
	var settings []*models.Setting
	err := s.db.NewSelect().
		Model(&settings).
		OrderExpr("category ASC, key ASC").
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return settings, nil
}

// Delete deletes a setting
func (s *BunSettingsStore) Delete(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Table("settings").
		Where("id = ?", id).
		Exec(ctx)

	return err
}
