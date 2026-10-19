package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// cleanupRetention binds each deletion to its window; the store deletes what
// finished before the cutoff.
type CleanupRetention struct {
	Store *Store
}

func (r CleanupRetention) DeleteFinishedStudentDocumentCleanups(ctx context.Context) (int, int, error) {
	return r.deleteFinished(ctx, domain.StudentDocumentCleanupRetentionWindow, r.Store.DeleteFinishedStudentDocumentFileCleanups)
}
func (r CleanupRetention) DeleteFinishedWithdrawalCompletions(ctx context.Context) (int, int, error) {
	return r.deleteFinished(ctx, domain.WithdrawalCompletionRetentionWindow, r.Store.DeleteFinishedWithdrawalCompletions)
}
func (r CleanupRetention) deleteFinished(ctx context.Context, window time.Duration, deleteBefore func(context.Context, time.Duration) (int, error)) (int, int, error) {
	deleted, err := deleteBefore(ctx, window)
	if err != nil {
		return 0, 0, err
	}
	return deleted, int(window.Hours() / 24), nil
}
