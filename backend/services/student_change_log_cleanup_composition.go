package services

import (
	"context"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/services/config"
)

// NewStudentChangeLogCleanup binds People Directory's change-history retention
// sweep (#1455) to the retained change-log table, the GDPR deletion trail and
// the tenant's retention setting. A nil deletion repository or settings
// service leaves the matching port unbound, so the sweep refuses to run
// instead of deleting without a trail or a verified retention period.
func NewStudentChangeLogCleanup(
	edits auditModels.StudentFieldEditRepository,
	deletions auditModels.DataDeletionRepository,
	settings config.SettingsService,
	logger *slog.Logger,
) peopleCompose.StudentChangeLogCleanupService {
	var deletionLog peopleCompose.StudentChangeLogDeletionLog
	if deletions != nil {
		deletionLog = func(ctx context.Context, entry peopleCompose.StudentChangeLogDeletion) error {
			deletion := auditModels.NewDataDeletion(
				entry.StudentID,
				auditModels.DeletionTypeStudentChangeLogRetention,
				entry.EditsDeleted,
				"system",
			)
			deletion.SetTenantID(entry.TenantID)
			deletion.DeletionReason = "automated student change-log retention cleanup"
			deletion.SetMetadata("retention_days", entry.RetentionDays)
			deletion.SetMetadata("cutoff", entry.Cutoff)
			return deletions.Create(ctx, deletion)
		}
	}
	var retention peopleCompose.StudentChangeLogRetention
	if settings != nil {
		retention = func(ctx context.Context) (int, error) {
			return settings.ResolveInt(ctx, configModels.KeyGDPRStudentChangeLogRetentionDays)
		}
	}
	return peopleCompose.NewStudentChangeLogCleanupService(edits, deletionLog, retention, logger)
}
