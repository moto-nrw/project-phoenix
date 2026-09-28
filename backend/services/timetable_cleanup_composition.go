package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// The Timetable retention contract and its result shapes, exposed to CLI
// composition the way the time-tracking cleanup is: the CLI composes through
// this root instead of importing the Timetable module (#3551).
type TimetableCleanup = timetable.TimetableCleanup
type TimetableCleanupResult = timetable.TimetableCleanupResult
type TimetableCleanupPreview = timetable.TimetableCleanupPreview
type TimetableCleanupStats = timetable.TimetableCleanupStats

// timetableRetentionSettingsSource is the slice of the settings service the
// retention window resolves through.
type timetableRetentionSettingsSource interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// timetableRetentionInputs are the collaborators the Timetable owner may not
// name itself: the Settings Platform's retention window and the Audit
// Platform's deletion records. Deviations and Settings are optional.
type timetableRetentionInputs struct {
	Owner      timetable.Capability
	Deletions  auditModels.DataDeletionRepository
	Deviations auditModels.DeviationEventRepository
	Settings   timetableRetentionSettingsSource
	Logger     *slog.Logger
	Clock      func() time.Time
}

// newTimetableCleanup composes the Timetable owner's GDPR retention (WP-B14).
func newTimetableCleanup(in timetableRetentionInputs) (timetable.TimetableCleanup, error) {
	logger := in.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps := timetableCompose.TimetableCleanupDependencies{
		Timetable: in.Owner,
		Logger:    logger,
		Clock:     in.Clock,
	}
	if in.Deletions != nil {
		deps.Audit = timetableDeletionAudit{deletions: in.Deletions}
	}
	if in.Deviations != nil {
		deps.DeviationEvents = timetableDeviationRetention{events: in.Deviations}
	}
	if in.Settings != nil {
		deps.Settings = timetableRetentionSettings{settings: in.Settings, logger: logger}
	}
	return timetableCompose.NewTimetableCleanup(deps)
}

// timetableRetentionSettings resolves gdpr.timetable_retention_days: tenant
// override, else the registry default. A failed override check is only
// logged; the resolution below still answers.
type timetableRetentionSettings struct {
	settings timetableRetentionSettingsSource
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
	deletions auditModels.DataDeletionRepository
}

func (a timetableDeletionAudit) RecordTimetableRetention(ctx context.Context, record timetableCompose.StudentDeletionRecord) error {
	deletion := auditModels.NewDataDeletion(
		record.StudentID,
		auditModels.DeletionTypeTimetableRetention,
		record.RecordsDeleted,
		"system",
	)
	deletion.SetTenantID(record.TenantID)
	deletion.DeletionReason = "automated timetable retention cleanup"
	deletion.SetMetadata("retention_days", record.RetentionDays)
	deletion.SetMetadata("cutoff_date", record.CutoffDate.String())
	if len(record.InstanceIDsSample) > 0 {
		deletion.SetMetadata("instance_ids_sample", record.InstanceIDsSample)
	}
	return a.deletions.Create(ctx, deletion)
}

// timetableDeviationRetention deletes the Änderungsprotokoll rows of the
// expired slots.
type timetableDeviationRetention struct {
	events auditModels.DeviationEventRepository
}

func (r timetableDeviationRetention) DeleteDeviationEventsBefore(ctx context.Context, cutoff timezone.Date) (int64, error) {
	return r.events.DeleteOlderThan(ctx, auditModels.Date(cutoff))
}
