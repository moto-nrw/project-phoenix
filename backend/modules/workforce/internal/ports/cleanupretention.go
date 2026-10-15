package ports

import (
	"context"
	"time"
)

// CleanupRetentionStore deletes the finished cleanup jobs of one school that
// finished before the instant (#4046). Every statement runs on the school's
// tenant transaction.
type CleanupRetentionStore interface {
	DeleteFinishedStaffDocumentFileCleanups(ctx context.Context, before time.Time) (int, error)
	DeleteCompletedOffboardingCleanups(ctx context.Context, before time.Time) (int, error)
}
