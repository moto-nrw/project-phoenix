package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
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

func NewSessionCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger) studentpresence.Presence {
	repos := NewSessionCleanupRepositories(db, timetableCapability)
	groups, supervisors := presenceCompose.SessionRepositories(repos.Sessions)
	settings := NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger)
	return presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: AttendancePrincipal,
		SchoolPresence:  NewLoggedStudentPresence(db, logger),
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
		NewLoggedStudentPresence(db, logger), supervisors, NewDeletionAudit(repos.Deletion),
		PresenceSettings(NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger)),
	)
}

func NewTimetableCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger, command AuditCommand) (TimetableCleanup, error) {
	audits := cleanupAuditRepositories(db, command)
	return NewTimetableCleanup(TimetableRetentionInputs{
		Owner:      timetableCapability,
		Deletions:  audits.DataDeletion,
		Deviations: audits.DeviationEvent,
		Settings:   NewCleanupSettingsService(settingsCompose.NewTenantSettingsRuntime(db, &runtime), schools, logger),
		Logger:     logger,
	})
}

// NewTimeTrackingCleanupService builds the time-tracking retention over the
// Workforce owner the command root composed; the deletion ledger comes from
// the Audit composition.
func NewTimeTrackingCleanupService(db *bun.DB, runtime tenant.UnitOfWork, schools organizationtenancy.Capability, workTime workforce.Capability, logger *slog.Logger, command AuditCommand) workforceCompose.TimeTrackingCleanupService {
	service, err := workforceCompose.NewTimeTrackingCleanup(workforceCompose.TimeTrackingCleanupDependencies{
		Workforce: workTime, Absences: workforceCompose.NewStaffAbsenceRepository(workTime), Audit: NewTimeTrackingRetentionAudit(cleanupAuditRepositories(db, command).DataDeletion),
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
