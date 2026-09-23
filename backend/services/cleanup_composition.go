package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
)

type School = organizationtenancy.School
type SchoolQuery = organizationtenancy.Query
type SchoolCapability = organizationtenancy.Capability
type TimetableCapability = timetable.Capability

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

// AuthMaintenance is the identity maintenance the cleanup CLI and the
// scheduler run: the expired session sweep and the revocation follow-ups
// (#3251) and the password reset maintenance (#3332), both called on the
// Identity & Access module directly (#3364).
type AuthMaintenance struct {
	identityaccess.AccountSessionMaintenance
	identityaccess.PasswordResets
}

// AuthMaintenanceRuntime is the maintenance the worker root schedules. It
// pairs the retained session sweep with the module's reset maintenance, so
// the worker keeps one identity dependency.
func (f *Factory) AuthMaintenanceRuntime() *AuthMaintenance {
	if f.Auth == nil {
		return nil
	}
	resets := f.AccountAuthentication()
	if resets == nil {
		return nil
	}
	return &AuthMaintenance{AccountSessionMaintenance: resets, PasswordResets: resets}
}

// NewAuthCleanupService composes the token and rate-limit maintenance the
// cleanup CLI runs. The expired session sweep and the revocation follow-ups
// are Identity & Access flows (#3251); the module is composed with the
// cleanup repositories and a signer the sweep never uses.
func NewAuthCleanupService(db *bun.DB, runtime tenant.UnitOfWork, logger *slog.Logger, command AuditCommand) (*AuthMaintenance, error) {
	repos := repositories.NewAuthCleanupRepositories(db, command)
	tokenAuth, err := configuredTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("auth cleanup service: token auth: %w", err)
	}
	codec, err := signedIdentityTokensOf(tokenAuth)
	if err != nil {
		return nil, err
	}
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositories{
			schools: schoolDirectory{schools: repos.School}, persons: repos.Person, authEvents: repos.AuthEvent, pushSubscriptions: repos.PushSubscription,
		},
		codec: codec, audit: command, logger: logger,
		// The cleanup root only removes spent links and stale windows; it
		// never issues a link, so it composes the flows without a mailer.
		resets: &passwordResetWiring{expiry: cleanupResetExpiry},
	})
	if err != nil {
		return nil, err
	}
	if err := identityAccess.SetTenantRuntime(runtime); err != nil {
		return nil, err
	}
	return &AuthMaintenance{AccountSessionMaintenance: identityAccess, PasswordResets: identityAccess}, nil
}

// NewInvitationCleanupService composes the invitation maintenance the
// cleanup CLI runs. The invitation flows themselves stay unavailable: the
// CLI only deletes expired links (#2722).
func NewInvitationCleanupService(db *bun.DB, logger *slog.Logger) (identityaccess.SchoolInvitations, error) {
	module, err := identityaccessCompose.New(identityaccessCompose.Dependencies{
		DB: db, Observe: func(identityaccessCompose.Observation) {},
	})
	if err != nil {
		return nil, fmt.Errorf("invitation cleanup service: %w", err)
	}
	return module, nil
}

func NewSessionCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger) studentpresence.Presence {
	repos := repositories.NewSessionCleanupRepositories(db, timetableCapability)
	groups, supervisors := presenceCompose.SessionRepositories(repos.Sessions)
	settings := NewCleanupSettingsService(db, runtime, schools, logger)
	return presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: AttendancePrincipal,
		SchoolPresence:  newStudentPresence(db, logger),
		GroupRepo:       groups, SupervisorRepo: supervisors,
		DeviceRepo: NewSessionDeviceDirectory(repos.Device, settings, logger), TimetableBridgeCompleter: repos.TimetableBridge, DB: db, Logger: logger,
	}, presenceservice.WithPresenceTenantRuntime(runtime), presenceservice.WithPresenceSettings(PresenceSettings(settings)))
}

func NewRetentionCleanupService(db *bun.DB, logger *slog.Logger, command AuditCommand) studentpresence.PresenceCleanup {
	repos := repositories.NewRetentionCleanupRepositories(db, command)
	_, supervisors := presenceCompose.SessionRepositories(repos.Sessions)
	return presenceservice.NewPresenceCleanup(
		newStudentPresence(db, logger), supervisors, NewDeletionAudit(repos.Deletion),
	)
}

func NewTimetableCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger, command AuditCommand) timetableplanning.TimetableCleanupService {
	repos := repositories.NewTimetableCleanupRepositories(db, command, timetableCapability)
	return timetableplanning.NewTimetableCleanupService(
		repos.Instance, repos.Exception, repos.Student, repos.Deletion, repos.Deviation,
		NewCleanupSettingsService(db, runtime, schools, logger), logger,
	)
}

func NewTimeTrackingCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger, command AuditCommand) timetracking.TimeTrackingCleanupService {
	repos := repositories.NewTimeTrackingCleanupRepositories(db, command)
	return timetracking.NewTimeTrackingCleanupService(
		repos.Session, repos.Absence, NewTimeTrackingRetentionAudit(repos.Deletion), PresenceSettings(NewCleanupSettingsService(db, runtime, schools, logger)), logger,
	)
}

func NewSettingsCommandRepository(db *bun.DB) configModels.SettingValueRepository {
	return repositories.NewSettingsCommandRepository(db)
}

// The time-tracking cleanup contract and its result shapes, exposed to CLI
// composition the way AuditCommand is: the CLI keeps composing through this
// root instead of importing the retained Workforce package.
type TimeTrackingCleanupService = timetracking.TimeTrackingCleanupService
type TimeTrackingCleanupResult = timetracking.TimeTrackingCleanupResult
type TimeTrackingCleanupPreview = timetracking.TimeTrackingCleanupPreview
type TimeTrackingCleanupStats = timetracking.TimeTrackingCleanupStats

// configuredTokenAuth resolves the signer of roots that receive no resolved
// configuration: the cleanup CLI and the test compositions.
func configuredTokenAuth() (*authjwt.TokenAuth, error) {
	return authjwt.NewTokenAuthWithDurations(
		viper.GetString("auth_jwt_secret"),
		viper.GetDuration("auth_jwt_expiry"),
		viper.GetDuration("auth_jwt_refresh_expiry"),
	)
}
