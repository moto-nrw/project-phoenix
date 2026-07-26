package config

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

const (
	tableSettingValues      = "config.setting_values"
	tableSettingValuesAlias = `config.setting_values AS "setting_value"`
)

// SettingValueRepository implements config.SettingValueRepository.
type SettingValueRepository struct {
	db *bun.DB
}

// NewSettingValueRepository creates a new SettingValueRepository.
func NewSettingValueRepository(db *bun.DB) config.SettingValueRepository {
	return &SettingValueRepository{db: db}
}

// FindByTenantAndKey retrieves a single value for a tenant and key.
// Returns (nil, nil) if not found.
func (r *SettingValueRepository) FindByTenantAndKey(ctx context.Context, tenantID int64, key string) (*config.SettingValue, error) {
	sv := new(config.SettingValue)
	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(sv).
		ModelTableExpr(tableSettingValuesAlias).
		Where(`"setting_value".tenant_id = ?`, tenantID).
		Where(`"setting_value".setting_key = ?`, key).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find setting value by tenant and key",
			Err: err,
		}
	}
	return sv, nil
}

// Upsert inserts or updates a setting value.
func (r *SettingValueRepository) Upsert(ctx context.Context, sv *config.SettingValue) error {
	if sv == nil {
		return fmt.Errorf("setting value cannot be nil")
	}
	if err := sv.Validate(); err != nil {
		return err
	}

	_, err := repoBase.GetDB(ctx, r.db).NewInsert().
		Model(sv).
		ModelTableExpr(tableSettingValues).
		On("CONFLICT (tenant_id, setting_key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "upsert setting value",
			Err: err,
		}
	}
	return nil
}

// Delete removes a single setting value for a tenant and key.
func (r *SettingValueRepository) Delete(ctx context.Context, tenantID int64, key string) error {
	_, err := repoBase.GetDB(ctx, r.db).NewDelete().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(tableSettingValues).
		Where("tenant_id = ?", tenantID).
		Where("setting_key = ?", key).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete setting value",
			Err: err,
		}
	}
	return nil
}
