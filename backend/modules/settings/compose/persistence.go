package compose

import (
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

// NewSettingsCommandRepository builds the setting value store for the
// privileged settings CLI, which runs outside a tenant transaction.
func NewSettingsCommandRepository(db *bun.DB) configModel.SettingValueRepository {
	return configRepo.NewSettingValueRepository(configRepo.NewRuntime(db))
}
