package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

// The cleanup command composes its sweeps here (#4013). Each sweep binds the
// owners it cleans, the tenant settings it reads its retention from and the
// Audit trail it records its deletions in.

func newSessionCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger) studentpresence.Presence {
	sessions, devices, completion := api.NewSessionCleanupRepositories(db, timetableCapability)
	groups, supervisors := presenceCompose.SessionRepositories(sessions)
	settings := newCleanupSettingsService(db, schools, logger)
	return presenceservice.NewPresence(presenceservice.PresenceDependencies{
		PrincipalReader: api.AttendancePrincipal,
		SchoolPresence:  owners.NewLoggedStudentPresence(db, logger),
		GroupRepo:       groups, SupervisorRepo: supervisors,
		DeviceRepo: owners.NewSessionDeviceDirectory(devices, settings, logger), TimetableBridgeCompleter: completion, DB: db.Pool(), Logger: logger,
	}, presenceservice.WithPresenceTenantRuntime(db.UnitOfWork()), presenceservice.WithPresenceSettings(settingsCompose.PresenceSettings(settings)))
}

func newRetentionCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, logger *slog.Logger, command auditModels.Command) studentpresence.PresenceCleanup {
	repos := api.NewRetentionCleanupRepositories(db, command)
	_, supervisors := presenceCompose.SessionRepositories(repos.Sessions)
	return presenceservice.NewPresenceCleanup(
		owners.NewLoggedStudentPresence(db, logger), supervisors, owners.NewDeletionAudit(repos.Deletion),
		settingsCompose.PresenceSettings(newCleanupSettingsService(db, schools, logger)),
	)
}

func newTimetableCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, timetableCapability timetable.Capability, logger *slog.Logger, command auditModels.Command) (timetable.TimetableCleanup, error) {
	audits := newCleanupAudits(db, command)
	return owners.NewTimetableCleanup(timetableCapability, audits.DataDeletion, audits.DeviationEvent,
		newCleanupSettingsService(db, schools, logger), logger, nil)
}

// newTimeTrackingCleanupService builds the time-tracking retention over the
// Workforce owner the command root composed; the deletion ledger comes from
// the Audit composition.
func newTimeTrackingCleanupService(db transactionruntime.Database, schools organizationtenancy.Capability, workTime workforce.Capability, logger *slog.Logger, command auditModels.Command) workforceCompose.TimeTrackingCleanupService {
	service, err := workforceCompose.NewTimeTrackingCleanup(workforceCompose.TimeTrackingCleanupDependencies{
		Workforce: workTime, Absences: workTime, Audit: owners.NewTimeTrackingRetentionAudit(newCleanupAudits(db, command).DataDeletion),
		Settings: settingsCompose.PresenceSettings(newCleanupSettingsService(db, schools, logger)), Logger: logger,
	})
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup: %v", err))
	}
	return service
}

// newCleanupSettingsService composes only the read/write adapters required
// by tenant-scoped retention and session cleanup commands, over the settings
// runtime bound to the command's database and unit of work.
func newCleanupSettingsService(db transactionruntime.Database, schools organizationtenancy.Capability, logger *slog.Logger) settingsCompose.SettingsService {
	runtime := db.UnitOfWork()
	settingsRuntime := settingsCompose.NewTenantSettingsRuntime(db.Pool(), &runtime)
	return settingsCompose.NewSettingsService(
		settingsCompose.NewSettingValueRepository(settingsRuntime),
		settingsCompose.NewSettingAuditRepository(settingsRuntime),
		owners.NewSchoolSettingsStore(schools),
		settingsRuntime,
		logger,
	)
}

// listRecentAuditRetentionSummaries reads the Audit-owned cleanup query
// without widening the producer-facing deletion repository. The summaries
// read the root database outside any transaction, scoped by the Audit tenant
// handoff.
func listRecentAuditRetentionSummaries(ctx context.Context, db transactionruntime.Database, since time.Time, limit int) ([]auditModels.RecentDeletionSummary, error) {
	audits, err := auditCompose.NewPersistence(auditCompose.PersistenceDependencies{
		Reads: auditCompose.NewFixedRuntime(db.Pool(), auditModels.TenantIDFromContext),
	})
	if err != nil {
		return nil, err
	}
	return audits.RetentionSummaries.ListRecentRetentionSummaries(ctx, since, limit)
}

// newCleanupAudits composes the Audit repositories a command root reads
// through the Audit handoff on the context. Auth events and deletions write
// through the same Audit command as the full service graph; the deviation
// protocol is only read and deleted in lockstep (#3551).
func newCleanupAudits(db transactionruntime.Database, command auditModels.Command) cleanupAudits {
	audits := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	deviations := audits.DeviationEvent
	audits = audits.WithRoutedWrites(command)
	return cleanupAudits{AuthEvent: audits.AuthEvent, DataDeletion: audits.DataDeletion, DeviationEvent: deviations}
}

type cleanupAudits struct {
	AuthEvent      auditModels.AuthEventRepository
	DataDeletion   auditModels.DataDeletionRepository
	DeviationEvent auditModels.DeviationEventRepository
}
