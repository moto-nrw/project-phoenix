package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/viper"
	"github.com/uptrace/bun"
)

type School = organizationtenancy.School
type SchoolQuery = organizationtenancy.Query
type SchoolCapability = organizationtenancy.Capability
type TimetableCapability = timetable.Capability

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
		auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{Reads: runtime}).AppendStore,
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
	repos := NewAuthCleanupRepositories(db)
	audits := cleanupAuditRepositories(db, command)
	tokenAuth, err := configuredTokenAuth()
	if err != nil {
		return nil, fmt.Errorf("auth cleanup service: token auth: %w", err)
	}
	codec, err := signedIdentityTokensOf(tokenAuth)
	if err != nil {
		return nil, err
	}
	identityAccess, err := newIdentityAccessWithSessions(identityaccessCompose.Dependencies{DB: db}, accountAuthenticationWiring{
		repos: sessionRepositories{
			schools: schoolDirectory{schools: repos.School}, persons: repos.Person, authEvents: audits.AuthEvent, pushSubscriptions: repos.PushSubscription,
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
	repos := NewSessionCleanupRepositories(db, timetableCapability)
	groups, supervisors := presenceCompose.SessionRepositories(repos.Sessions)
	settings := NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger)
	return presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: AttendancePrincipal,
		SchoolPresence:  newStudentPresence(db, logger),
		GroupRepo:       groups, SupervisorRepo: supervisors,
		DeviceRepo: NewSessionDeviceDirectory(repos.Device, settings, logger), TimetableBridgeCompleter: repos.TimetableBridge, DB: db, Logger: logger,
	}, presenceservice.WithPresenceTenantRuntime(runtime), presenceservice.WithPresenceSettings(PresenceSettings(settings)))
}

// RetentionCleanupRepositories are the Student Presence session and
// supervision records the stale-supervisor cleanup runs on and the deletion
// ledger it records into.
type RetentionCleanupRepositories struct {
	Sessions studentpresence.SessionRecords
	Deletion auditModels.DataDeletionRepository
}

// NewRetentionCleanupRepositories composes the retention cleanup's records
// with the deletion ledger routed through command.
func NewRetentionCleanupRepositories(db *bun.DB, command AuditCommand) RetentionCleanupRepositories {
	return RetentionCleanupRepositories{
		Sessions: NewPresenceSessionRecords(db),
		Deletion: cleanupAuditRepositories(db, command).DataDeletion,
	}
}

func NewRetentionCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger, command AuditCommand) studentpresence.PresenceCleanup {
	repos := NewRetentionCleanupRepositories(db, command)
	_, supervisors := presenceCompose.SessionRepositories(repos.Sessions)
	return presenceservice.NewPresenceCleanup(
		newStudentPresence(db, logger), supervisors, NewDeletionAudit(repos.Deletion),
		PresenceSettings(NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger)),
	)
}

func NewTimetableCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger, command AuditCommand) (TimetableCleanup, error) {
	audits := cleanupAuditRepositories(db, command)
	return newTimetableCleanup(timetableRetentionInputs{
		Owner:      timetableCapability,
		Deletions:  audits.DataDeletion,
		Deviations: audits.DeviationEvent,
		Settings:   NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger),
		Logger:     logger,
	})
}

func NewTimeTrackingCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, logger *slog.Logger, command AuditCommand) workforceCompose.TimeTrackingCleanupService {
	repos := NewTimeTrackingCleanupRepositories(db)
	service, err := workforceCompose.NewTimeTrackingCleanup(workforceCompose.TimeTrackingCleanupDependencies{
		Workforce: repos.Workforce, Absences: repos.Absence, Audit: NewTimeTrackingRetentionAudit(cleanupAuditRepositories(db, command).DataDeletion),
		Settings: PresenceSettings(NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger)), Logger: logger,
	})
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup: %v", err))
	}
	return service
}

func NewSettingsCommandRepository(db *bun.DB) configModels.SettingValueRepository {
	return settingsCompose.NewSettingsCommandRepository(db)
}

// ListRecentAuditRetentionSummaries exposes the Audit-owned cleanup query to
// the cleanup CLI without widening the producer-facing deletion repository.
func ListRecentAuditRetentionSummaries(ctx context.Context, db *bun.DB, since time.Time, limit int) ([]auditModels.RecentDeletionSummary, error) {
	// The summaries read the root database outside any transaction, scoped by
	// the Audit tenant handoff.
	reads := func(ctx context.Context) (bun.IDB, int64) { return db, auditModels.TenantIDFromContext(ctx) }
	audits, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{Reads: reads})
	if err != nil {
		return nil, err
	}
	return audits.RetentionSummaries.ListRecentRetentionSummaries(ctx, since, limit)
}

// cleanupAuditRepositories composes the Audit repositories a command root
// reads through the Audit handoff on the context. Auth events and deletions
// write through the same Audit command as the full service graph; the
// deviation protocol is only read and deleted in lockstep (#3551).
func cleanupAuditRepositories(db *bun.DB, command AuditCommand) cleanupAudits {
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db})
	deviations := audits.DeviationEvent
	audits = audits.WithRoutedWrites(command)
	return cleanupAudits{AuthEvent: audits.AuthEvent, DataDeletion: audits.DataDeletion, DeviationEvent: deviations}
}

type cleanupAudits struct {
	AuthEvent      auditModels.AuthEventRepository
	DataDeletion   auditModels.DataDeletionRepository
	DeviationEvent auditModels.DeviationEventRepository
}

// The time-tracking cleanup contract and its result shapes, exposed to CLI
// composition the way AuditCommand is: the CLI keeps composing through this
// root instead of importing the retained Workforce package.
type TimeTrackingCleanupService = workforceCompose.TimeTrackingCleanupService
type TimeTrackingCleanupResult = workforceCompose.TimeTrackingCleanupResult
type TimeTrackingCleanupPreview = workforceCompose.TimeTrackingCleanupPreview
type TimeTrackingCleanupStats = workforceCompose.TimeTrackingCleanupStats

// signedIdentityTokensOf adapts a root's resolved signer to the native claim
// codec. The signing primitive stays in the token adapter; Identity owns the
// claim schema and validation.
func signedIdentityTokensOf(signer *authjwt.TokenAuth) (identityaccessCompose.SignedIdentityTokens, error) {
	return NewSessionTokens(signer.JwtAuth, signer.JwtExpiry, signer.JwtRefreshExpiry)
}

// configuredTokenAuth resolves the signer of roots that receive no resolved
// configuration: the cleanup CLI and the test compositions.
func configuredTokenAuth() (*authjwt.TokenAuth, error) {
	return authjwt.NewTokenAuthWithDurations(
		viper.GetString("auth_jwt_secret"),
		viper.GetDuration("auth_jwt_expiry"),
		viper.GetDuration("auth_jwt_refresh_expiry"),
	)
}
