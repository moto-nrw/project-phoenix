package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewCleanupSettingsService composes only the read/write adapters required by
// tenant-scoped retention and session cleanup commands.
func NewCleanupSettingsService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger) settingsCompose.SettingsService {
	settingsRuntime := newSettingsRuntime(db, &runtime)
	return settingsCompose.NewSettingsService(
		settingsCompose.NewSettingValueRepository(settingsRuntime),
		settingsCompose.NewSettingAuditRepository(settingsRuntime),
		newSchoolSettingsStore(schools),
		settingsRuntime,
		logger,
	)
}
