package scheduler

import "context"

// TimeTrackingCleanupService is the scheduler's port to the Workforce GDPR
// cleanup. The composition root binds the retained time-tracking cleanup; the
// scheduler only needs the per-tenant run and the counts it logs.
type TimeTrackingCleanupService interface {
	CleanupExpiredTimeTrackingData(ctx context.Context) (*TimeTrackingCleanupResult, error)
}

// TimeTrackingCleanupResult summarises one tenant's cleanup run.
type TimeTrackingCleanupResult struct {
	SessionsDeleted int
	AbsencesDeleted int
	StaffAffected   int
	RetentionDays   int
	DurationMS      int64
}
