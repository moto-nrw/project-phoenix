package compose

import (
	"context"
	"time"
)

// StudentConsentHistory is the Audit Platform seam behind the shared consent
// projection. The composition root binds it: Audit Platform owns the consent
// trail, People Directory only folds it together with the live timestamps.
type StudentConsentHistory interface {
	// LatestPhotoWithdrawal returns when the child's photo consent was last
	// withdrawn, or nil when the latest recorded photo event is not a
	// withdrawal.
	LatestPhotoWithdrawal(ctx context.Context, studentID int64) (*time.Time, error)
}
