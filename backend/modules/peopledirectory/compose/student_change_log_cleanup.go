package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// NewStudentChangeLogCleanupService constructs the GDPR retention cleanup of
// the per-child change history (#1455). deletions receives one entry per
// affected student. Cleanup fails closed when the retention setting is not
// wired, so it never applies an unverified retention policy.
func NewStudentChangeLogCleanupService(
	store ports.StudentChangeLogStore,
	deletions ports.StudentChangeLogDeletionLog,
	retention ports.StudentChangeLogRetention,
	logger *slog.Logger,
) ports.StudentChangeLogCleanupService {
	return application.NewStudentChangeLogCleanupService(transaction{}, store, deletions, retention, logger)
}
