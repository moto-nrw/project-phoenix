package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
)

// NewCleanupSettingsService composes only the read/write adapters required by
// tenant-scoped retention and session cleanup commands, over the settings
// runtime the caller binds to its database and unit of work.
func NewCleanupSettingsService(settingsRuntime settingsCompose.TenantSettingsRuntime, schools organizationtenancy.Capability, logger *slog.Logger) settingsCompose.SettingsService {
	return settingsCompose.NewSettingsService(
		settingsCompose.NewSettingValueRepository(settingsRuntime),
		settingsCompose.NewSettingAuditRepository(settingsRuntime),
		newSchoolSettingsStore(schools),
		settingsRuntime,
		logger,
	)
}
