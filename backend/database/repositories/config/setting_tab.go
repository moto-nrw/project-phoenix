package config

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// SettingTabRepository implements config.SettingTabRepository
type SettingTabRepository struct {
	db *bun.DB
}

// NewSettingTabRepository creates a new setting tab repository
func NewSettingTabRepository(db *bun.DB) *SettingTabRepository {
	return &SettingTabRepository{db: db}
}

// Create inserts a new tab
func (r *SettingTabRepository) Create(ctx context.Context, tab *config.SettingTab) error {
	if err := tab.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(tab).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Returning("*").
		Exec(ctx)
	return err
}

// Update modifies an existing tab
func (r *SettingTabRepository) Update(ctx context.Context, tab *config.SettingTab) error {
	if err := tab.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewUpdate().
		Model(tab).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Where(`"setting_tab".id = ?`, tab.ID).
		Where(`"setting_tab".deleted_at IS NULL`).
		Returning("*").
		Exec(ctx)
	return err
}

// FindByID retrieves a tab by ID (excludes soft-deleted)
func (r *SettingTabRepository) FindByID(ctx context.Context, id int64) (*config.SettingTab, error) {
	var tab config.SettingTab
	err := r.db.NewSelect().
		Model(&tab).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Where(`"setting_tab".id = ?`, id).
		Where(`"setting_tab".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &tab, nil
}

// FindByKey retrieves a tab by key (excludes soft-deleted)
func (r *SettingTabRepository) FindByKey(ctx context.Context, key string) (*config.SettingTab, error) {
	var tab config.SettingTab
	err := r.db.NewSelect().
		Model(&tab).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Where(`"setting_tab".key = ?`, key).
		Where(`"setting_tab".deleted_at IS NULL`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &tab, nil
}

// FindAll retrieves all active tabs ordered by display_order
func (r *SettingTabRepository) FindAll(ctx context.Context) ([]*config.SettingTab, error) {
	var tabs []*config.SettingTab
	err := r.db.NewSelect().
		Model(&tabs).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Where(`"setting_tab".deleted_at IS NULL`).
		Order("display_order ASC", "name ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return tabs, nil
}

// SoftDelete marks a tab as deleted
func (r *SettingTabRepository) SoftDelete(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*config.SettingTab)(nil)).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Set("deleted_at = ?", now).
		Where(`"setting_tab".id = ?`, id).
		Where(`"setting_tab".deleted_at IS NULL`).
		Exec(ctx)
	return err
}

// Restore unmarks a soft-deleted tab
func (r *SettingTabRepository) Restore(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*config.SettingTab)(nil)).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		Set("deleted_at = NULL").
		Where(`"setting_tab".id = ?`, id).
		Where(`"setting_tab".deleted_at IS NOT NULL`).
		Exec(ctx)
	return err
}

// Upsert creates or updates a tab by key
func (r *SettingTabRepository) Upsert(ctx context.Context, tab *config.SettingTab) error {
	if err := tab.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(tab).
		ModelTableExpr(`config.setting_tabs AS "setting_tab"`).
		On("CONFLICT (key) WHERE deleted_at IS NULL DO UPDATE").
		Set("name = EXCLUDED.name").
		Set("icon = EXCLUDED.icon").
		Set("display_order = EXCLUDED.display_order").
		Set("required_permission = EXCLUDED.required_permission").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)
	return err
}
