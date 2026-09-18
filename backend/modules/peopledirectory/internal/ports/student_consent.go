package ports

import (
	"context"
	"time"
)

// StudentConsentHistory is the consumer-owned port over the append-only
// consent trail. Audit Platform owns audit.student_consent_changes; the
// directory only needs to know whether a child's photo consent was withdrawn
// after it was last granted.
type StudentConsentHistory interface {
	// LatestPhotoWithdrawal returns when the child's photo consent was last
	// withdrawn, or nil when the latest recorded photo event is not a
	// withdrawal.
	LatestPhotoWithdrawal(ctx context.Context, studentID int64) (*time.Time, error)
}
