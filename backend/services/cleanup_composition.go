package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type School = organizationtenancy.School
type SchoolQuery = organizationtenancy.Query
type SchoolCapability = organizationtenancy.Capability

func NewOrganizationTenancy(db *bun.DB) (SchoolCapability, error) {
	return repositories.NewOrganizationTenancy(db)
}

// NewCleanupAuditCommand builds the same fail-closed Audit command used by
// the HTTP service graph. Cleanup producers must already be inside their
// authoritative transaction when they append an event.
func NewCleanupAuditCommand(logger *slog.Logger) (AuditCommand, error) {
	if logger == nil {
		return nil, fmt.Errorf("cleanup audit command logger is required")
	}
	runtime := func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModels.TenantIDFromContext(ctx)
		raw, ok := auditModels.TransactionFromContext(ctx)
		if !ok {
			return nil, tenantID
		}
		switch tx := raw.(type) {
		case bun.Tx:
			return tx, tenantID
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID
			}
		}
		panic(fmt.Sprintf("audit command: unsupported transaction %T", raw))
	}
	return newAuditCommand(
		repositories.NewAuditStore(runtime),
		logger.With("component", "audit-command"),
		func(string, time.Duration, int, error) {},
	)
}

// AuditCommand exposes Audit's single command type to CLI composition without
// making the CLI import the Audit domain package directly.
type AuditCommand = auditModels.Command

func NewAuthCleanupService(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger, command AuditCommand) *auth.Service {
	repos := repositories.NewAuthCleanupRepositories(db, command)
	return auth.NewCleanupService(auth.CleanupDependencies{
		Account: repos.Account, Token: repos.Token, PasswordResetRateLimit: repos.PasswordResetRateLimit,
		AuthEvent: repos.AuthEvent, Audit: command, PushSubscription: repos.PushSubscription,
		DB: db, Logger: logger, TenantRuntime: runtime,
	})
}

func NewInvitationCleanupService(db *bun.DB, logger *slog.Logger) auth.InvitationService {
	return auth.NewInvitationService(auth.InvitationServiceConfig{
		InvitationRepo: repositories.NewInvitationCleanupRepository(db), DB: db, Logger: logger,
	})
}

func NewSessionCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger) active.Service {
	repos := repositories.NewSessionCleanupRepositories(db)
	service := active.NewService(active.ServiceDependencies{
		GroupRepo: repos.Group, VisitRepo: repos.Visit, SupervisorRepo: repos.Supervisor,
		DeviceRepo: repos.Device, TimetableBridgeCompleter: repos.TimetableBridge, DB: db, Logger: logger,
	})
	service.SetSettingsService(NewCleanupSettingsService(db, runtime, schools, logger))
	return service
}

func NewRetentionCleanupService(db *bun.DB, logger *slog.Logger, command AuditCommand) active.CleanupService {
	repos := repositories.NewRetentionCleanupRepositories(db, command)
	return active.NewCleanupService(
		repos.Visit, repos.Attendance, repos.Supervisor, repos.Consent, repos.Deletion,
		users.NewPrivacyConsentService(nil, logger), db,
	)
}

func NewTimetableCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger, command AuditCommand) schedule.TimetableCleanupService {
	repos := repositories.NewTimetableCleanupRepositories(db, command)
	return schedule.NewTimetableCleanupService(
		repos.Instance, repos.Exception, repos.Student, repos.Deletion, repos.Deviation,
		NewCleanupSettingsService(db, runtime, schools, logger), logger,
	)
}

func NewTimeTrackingCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger, command AuditCommand) active.TimeTrackingCleanupService {
	repos := repositories.NewTimeTrackingCleanupRepositories(db, command)
	return active.NewTimeTrackingCleanupService(
		repos.Session, repos.Absence, repos.Deletion, NewCleanupSettingsService(db, runtime, schools, logger), logger,
	)
}

func NewSettingsCommandRepository(db *bun.DB) configModels.SettingValueRepository {
	return repositories.NewSettingsCommandRepository(db)
}
