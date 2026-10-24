package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/uptrace/bun"
)

func NewDocumentCleanup(db *bun.DB, now func() time.Time) (*workforce.DocumentCleanup, error) {
	if db == nil {
		return nil, errors.New("document cleanup: database is required")
	}
	if now == nil {
		now = time.Now
	}
	store := postgres.New(postgres.DatabaseRuntime(db))
	service := application.NewDocumentCleanup(store, transaction{lock: store.AcquireXactLock}, now)
	return workforce.NewDocumentCleanup(service.Enqueue, service.Claim, service.Finish, service.Backlog), nil
}

// NewStaffDocumentFileRecovery builds the removal of stored staff document
// files over the document service, the offboarding cleanup queue and the
// upload directory the root binds. Every dependency is required.
func NewStaffDocumentFileRecovery(documents workforce.StaffDocumentFileCleanups, queue workforce.DocumentCleanupQueue, uploads workforce.StaffDocumentUploads, logger *slog.Logger) (workforce.StaffDocumentFileRecovery, error) {
	return application.NewStaffDocumentFileRecovery(documents, queue, uploads, logger)
}
