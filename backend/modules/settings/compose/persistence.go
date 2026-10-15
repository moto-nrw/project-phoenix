package compose

import (
	"context"
	"encoding/json"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/uptrace/bun"
)

// PersistenceRuntime supplies the ambient transaction the Settings Postgres
// adapter runs in; tenant scoping is left to row-level security.
type PersistenceRuntime = configRepo.Runtime

// NewSettingValueRepository builds the tenant setting value store over the
// caller's settings runtime (ADR 0049).
func NewSettingValueRepository(runtime PersistenceRuntime) configModel.SettingValueRepository {
	return configRepo.NewSettingValueRepository(runtime)
}

// NewSettingAuditRepository builds the setting change trail over the caller's
// settings runtime.
func NewSettingAuditRepository(runtime PersistenceRuntime) configModel.SettingAuditRepository {
	return configRepo.NewSettingAuditRepository(runtime)
}

// NewHomeLayoutRepository builds the store behind the personal start page
// (#2875). Callers pass the same tenant-aware runtime as the settings
// repositories, so its rows take part in the settings tenant transaction and
// stay behind RLS.
func NewHomeLayoutRepository(runtime PersistenceRuntime) configModel.HomeLayoutRepository {
	return configRepo.NewHomeLayoutRepository(runtime)
}

// StoredOverrideReader reads the overrides a school stored, without registry
// defaults. The privileged settings CLI lists them outside a tenant
// transaction.
type StoredOverrideReader struct {
	values configModel.SettingValueRepository
}

// NewStoredOverrideReader builds the reader over the CLI's superuser pool.
func NewStoredOverrideReader(db *bun.DB) StoredOverrideReader {
	return StoredOverrideReader{values: configRepo.NewSettingValueRepository(configRepo.NewRuntime(db))}
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
