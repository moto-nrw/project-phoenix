package workforce

import "context"

// CleanupRetention deletes the finished cleanup jobs of the school in context
// after the fixed window of the retention list (#4046): a staff document's
// file cleanup goes 30 days after its bytes were removed, an offboarding job
// 30 days after it completed. Jobs still running stay.
type CleanupRetention interface {
	// DeleteFinishedStaffCleanups reports the jobs it deleted and the window
	// in days.
	DeleteFinishedStaffCleanups(ctx context.Context) (deleted, retentionDays int, err error)
}
