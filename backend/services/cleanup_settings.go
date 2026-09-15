package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewCleanupSettingsService composes only the read/write adapters required by
// tenant-scoped retention and session cleanup commands.
func NewCleanupSettingsService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger) config.SettingsService {
	settingsRuntime := newSettingsRuntime(db, &runtime)
	repos := repositories.NewCleanupSettingsRepositories(db, settingsRuntime)
	return config.NewSettingsService(
		repos.Value,
		repos.Audit,
		newSchoolSettingsStore(schools),
		settingsRuntime,
		logger,
	)
}
