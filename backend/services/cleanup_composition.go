package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewAuthCleanupService(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger) *auth.Service {
	repos := repositories.NewAuthCleanupRepositories(db)
	return auth.NewCleanupService(auth.CleanupDependencies{
		Account: repos.Account, Token: repos.Token, PasswordResetRateLimit: repos.PasswordResetRateLimit,
		AuthEvent: repos.AuthEvent, PushSubscription: repos.PushSubscription,
		DB: db, Logger: logger, TenantRuntime: runtime,
	})
}

func NewInvitationCleanupService(db *bun.DB, logger *slog.Logger) auth.InvitationService {
	return auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo: repositories.NewInvitationCleanupRepository(db), DB: db, Logger: logger,
	})
}

func NewSessionCleanupService(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger) active.Service {
	repos := repositories.NewSessionCleanupRepositories(db)
	service := active.NewService(active.ServiceDependencies{
		GroupRepo: repos.Group, VisitRepo: repos.Visit, SupervisorRepo: repos.Supervisor,
		DeviceRepo: repos.Device, TimetableBridgeCompleter: repos.TimetableBridge, DB: db, Logger: logger,
	})
	service.SetSettingsService(NewCleanupSettingsService(db, runtime, logger))
	return service
}

func NewRetentionCleanupService(db *bun.DB, logger *slog.Logger) active.CleanupService {
	repos := repositories.NewRetentionCleanupRepositories(db)
	return active.NewCleanupService(
		repos.Visit, repos.Attendance, repos.Supervisor, repos.Consent, repos.Deletion,
		users.NewPrivacyConsentService(nil, logger), db,
	)
}

func NewTimetableCleanupService(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger) schedule.TimetableCleanupService {
	repos := repositories.NewTimetableCleanupRepositories(db)
	return schedule.NewTimetableCleanupService(
		repos.Instance, repos.Exception, repos.Student, repos.Deletion, repos.Deviation,
		NewCleanupSettingsService(db, runtime, logger), logger,
	)
}

func NewTimeTrackingCleanupService(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger) active.TimeTrackingCleanupService {
	repos := repositories.NewTimeTrackingCleanupRepositories(db)
	return active.NewTimeTrackingCleanupService(
		repos.Session, repos.Absence, repos.Deletion, NewCleanupSettingsService(db, runtime, logger), logger,
	)
}

func NewSettingsCommandRepositories(db *bun.DB) (platformModels.SchoolRepository, configModels.SettingValueRepository) {
	return repositories.NewSettingsCommandRepositories(db)
}
