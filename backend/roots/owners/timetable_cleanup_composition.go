package owners

import (
	"context"
	"log/slog"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableCleanup composes the Timetable owner's GDPR retention (WP-B14)
// over the collaborators the owner may not name itself: the Audit Platform's
// deletion records and Änderungsprotokoll sweep and the Settings Platform's
// retention window. deviations and settings are optional; a nil logger is
// slog.Default().
func NewTimetableCleanup(owner timetable.Capability, deletions DataDeletionAppender, deviations timetableDeviationSweep, settings TimetableRetentionSettings, logger *slog.Logger, clock func() time.Time) (timetable.TimetableCleanup, error) {
	if logger == nil {
		logger = slog.Default()
	}
	deps := timetablePorts.TimetableCleanupDependencies{
		Timetable: owner,
		Logger:    logger,
		Clock:     clock,
	}
	if deletions != nil {
		deps.Audit = timetableDeletionAudit{deletions: deletions}
	}
	if deviations != nil {
		deps.DeviationEvents = timetableDeviationRetention{events: deviations}
	}
	if settings != nil {
		deps.Settings = timetableRetentionSettings{settings: settings, logger: logger}
	}
	return timetableCompose.NewTimetableCleanup(deps)
}

// timetableRetentionSettings resolves gdpr.timetable_retention_days: tenant
// override, else the registry default. A failed override check is only
// logged; the resolution below still answers.
type timetableRetentionSettings struct {
	settings TimetableRetentionSettings
	logger   *slog.Logger
}

func (s timetableRetentionSettings) TimetableRetentionDays(ctx context.Context) (int, error) {
	if _, err := s.settings.HasTenantOverride(ctx, configModels.KeyGDPRTimetableRetentionDays); err != nil {
		s.logger.Warn("settings override check failed, falling back to registry default",
			slog.String("key", configModels.KeyGDPRTimetableRetentionDays),
			slog.String("error", err.Error()),
		)
	}
	return s.settings.ResolveInt(ctx, configModels.KeyGDPRTimetableRetentionDays)
}

// timetableDeletionAudit writes one audit.data_deletions row per child the
// retention run touches.
type timetableDeletionAudit struct {
	deletions DataDeletionAppender
}

func (a timetableDeletionAudit) RecordTimetableRetention(ctx context.Context, record timetablePorts.StudentDeletionRecord) error {
	return a.deletions.Create(ctx, auditModels.NewTimetableRetentionDeletion(
		record.TenantID, record.StudentID, record.RecordsDeleted, record.RetentionDays,
		record.CutoffDate.String(), record.InstanceIDsSample,
	))
}

// timetableDeviationRetention deletes the Änderungsprotokoll rows of the
// expired slots.
type timetableDeviationRetention struct {
	events timetableDeviationSweep
}

func (r timetableDeviationRetention) DeleteDeviationEventsBefore(ctx context.Context, cutoff timezone.Date) (int64, error) {
	return r.events.DeleteOlderThan(ctx, auditModels.Date(cutoff))
}

// timetableDeviationSweep deletes the Änderungsprotokoll entries older than
// the retention cutoff.
type timetableDeviationSweep interface {
	DeleteOlderThan(ctx context.Context, cutoff auditModels.Date) (int64, error)
}
