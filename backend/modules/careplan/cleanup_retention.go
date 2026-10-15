package careplan

import "context"

// CleanupRetention deletes the finished records of the school in context
// after the fixed windows of the retention list (#4046). Records still open
// stay.
type CleanupRetention interface {
	// DeleteFinishedStudentDocumentCleanups deletes a child document's file
	// cleanup 30 days after its bytes were removed. It reports the intents
	// it deleted and the window in days.
	DeleteFinishedStudentDocumentCleanups(ctx context.Context) (deleted, retentionDays int, err error)
	// DeleteFinishedWithdrawalCompletions deletes a resolved or obsolete
	// care-withdrawal completion 365 days after it was finished; the
	// history then shows the last year. It reports the completions it
	// deleted and the window in days.
	DeleteFinishedWithdrawalCompletions(ctx context.Context) (deleted, retentionDays int, err error)
}
