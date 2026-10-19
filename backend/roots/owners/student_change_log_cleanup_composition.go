package owners

import (
	"context"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// NewStudentChangeLogCleanup binds People Directory's change-history retention
// sweep (#1455) to the retained change-log table, the GDPR deletion trail and
// the tenant's retention setting. A nil deletion repository or settings
// service leaves the matching port unbound, so the sweep refuses to run
// instead of deleting without a trail or a verified retention period.
func NewStudentChangeLogCleanup(
	edits peoplePorts.StudentChangeLogStore,
	deletions DataDeletionAppender,
	settings RetentionSettings,
	logger *slog.Logger,
) peoplePorts.StudentChangeLogCleanupService {
	var deletionLog peoplePorts.StudentChangeLogDeletionLog
	if deletions != nil {
		deletionLog = func(ctx context.Context, entry peoplePorts.StudentChangeLogDeletion) error {
			return deletions.Create(ctx, auditModels.NewStudentChangeLogRetentionDeletion(
				entry.TenantID, entry.StudentID, entry.EditsDeleted, entry.RetentionDays, entry.Cutoff,
			))
		}
	}
	var retention peoplePorts.StudentChangeLogRetention
	if settings != nil {
		retention = func(ctx context.Context) (int, error) {
			return settings.ResolveInt(ctx, configModels.KeyGDPRStudentChangeLogRetentionDays)
		}
	}
	return peopleCompose.NewStudentChangeLogCleanupService(edits, deletionLog, retention, logger)
}
