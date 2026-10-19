package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/uptrace/bun"
)

const (
	tableSettingValues      = "config.setting_values"
	tableSettingValuesAlias = `config.setting_values AS "setting_value"`
)

// SettingValueRepository implements config.SettingValueRepository.
type SettingValueRepository struct {
	runtime Runtime
}

// NewSettingValueRepository creates a new SettingValueRepository.
func NewSettingValueRepository(runtime Runtime) config.SettingValueRepository {
	return &SettingValueRepository{runtime: runtime}
}

// FindByTenantAndKey retrieves a single value for a tenant and key.
// Returns (nil, nil) if not found.
func (r *SettingValueRepository) FindByTenantAndKey(ctx context.Context, tenantID int64, key string) (*config.SettingValue, error) {
	sv := new(config.SettingValue)
	ambientDB1, databaseErr1 := r.runtime.DB(ctx)
	if databaseErr1 != nil {
		return *new(*config.SettingValue), databaseErr1
	}
	err := ambientDB1.NewSelect().
		Model(sv).
		ModelTableExpr(tableSettingValuesAlias).
		Where(`"setting_value".tenant_id = ?`, tenantID).
		Where(`"setting_value".setting_key = ?`, key).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find setting value by tenant and key: %w", err)
	}
	return sv, nil
}

// FindByTenantAndKeys retrieves the stored overrides for the named keys in one
// query. Registry defaults remain a service-layer concern.
func (r *SettingValueRepository) FindByTenantAndKeys(ctx context.Context, tenantID int64, keys []string) ([]*config.SettingValue, error) {
	if tenantID <= 0 || len(keys) == 0 {
		return nil, nil
	}

	var values []*config.SettingValue
	ambientDB2, databaseErr2 := r.runtime.DB(ctx)
	if databaseErr2 != nil {
		return *new([]*config.SettingValue), databaseErr2
	}
	err := ambientDB2.NewSelect().
		Model(&values).
		ModelTableExpr(tableSettingValuesAlias).
		Where(`"setting_value".tenant_id = ?`, tenantID).
		Where(`"setting_value".setting_key IN (?)`, bun.List(keys)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find setting values by tenant and keys: %w", err)
	}
	return values, nil
}

// FindByTenantsAndKeys retrieves stored overrides for several tenants and keys
// in one query. The service layer wraps this privileged read in an admin
// transaction; repository code deliberately does not change database roles.
func (r *SettingValueRepository) FindByTenantsAndKeys(ctx context.Context, tenantIDs []int64, keys []string) ([]*config.SettingValue, error) {
	if len(tenantIDs) == 0 || len(keys) == 0 {
		return nil, nil
	}

	var values []*config.SettingValue
	ambientDB3, databaseErr3 := r.runtime.DB(ctx)
	if databaseErr3 != nil {
		return *new([]*config.SettingValue), databaseErr3
	}
	err := ambientDB3.NewSelect().
		Model(&values).
		ModelTableExpr(tableSettingValuesAlias).
		Where(`"setting_value".tenant_id IN (?)`, bun.List(tenantIDs)).
		Where(`"setting_value".setting_key IN (?)`, bun.List(keys)).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("find setting values by tenants and keys: %w", err)
	}
	return values, nil
}

// Upsert inserts or updates a setting value.
func (r *SettingValueRepository) Upsert(ctx context.Context, sv *config.SettingValue) error {
	if sv == nil {
		return fmt.Errorf("setting value cannot be nil")
	}
	if err := sv.Validate(); err != nil {
		return err
	}

	ambientDB4, databaseErr4 := r.runtime.DB(ctx)
	if databaseErr4 != nil {
		return databaseErr4
	}
	_, err := ambientDB4.NewInsert().
		Model(sv).
		ModelTableExpr(tableSettingValues).
		On("CONFLICT (tenant_id, setting_key) DO UPDATE").
		Set("value = EXCLUDED.value").
		Set("updated_by = EXCLUDED.updated_by").
		Set("updated_at = NOW()").
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("upsert setting value: %w", err)
	}
	return nil
}

// Delete removes a single setting value for a tenant and key.
func (r *SettingValueRepository) Delete(ctx context.Context, tenantID int64, key string) error {
	ambientDB5, databaseErr5 := r.runtime.DB(ctx)
	if databaseErr5 != nil {
		return databaseErr5
	}
	_, err := ambientDB5.NewDelete().
		Model((*config.SettingValue)(nil)).
		ModelTableExpr(tableSettingValues).
		Where("tenant_id = ?", tenantID).
		Where("setting_key = ?", key).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("delete setting value: %w", err)
	}
	return nil
}

// StoredOverrideReader reads the overrides a school stored, without registry
// defaults, inside the caller's explicit school transaction.
type StoredOverrideReader struct {
	values config.SettingValueRepository
}

// NewStoredOverrideReader binds the stored-override query to its runtime.
func NewStoredOverrideReader(runtime Runtime) StoredOverrideReader {
	return StoredOverrideReader{values: NewSettingValueRepository(runtime)}
}

// StoredOverride returns the stored JSON value of key for the school, and
// false when the school stored none.
func (r StoredOverrideReader) StoredOverride(ctx context.Context, tenantID int64, key string) (json.RawMessage, bool, error) {
	value, err := r.values.FindByTenantAndKey(ctx, tenantID, key)
	if err != nil || value == nil {
		return nil, false, err
	}
	return value.Value, true, nil
}
