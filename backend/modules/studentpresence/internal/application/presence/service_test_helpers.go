//go:build testhooks

package presence

import (
	"context"
	"time"
)

// ConfigureForTest applies construction-time options to a service the
// behaviour tests already built through their shared fixtures. Production
// composition passes the same options to NewService; there is no setter.
func ConfigureForTest(svc Service, options ...ServiceOption) {
	target := svc.(*service)
	for _, option := range options {
		option(target)
	}
}

// ValidateTransferredSupervisorLocksForTest exercises the post-lock guard with
// real owner repositories without replaying the preceding session writes.
func ValidateTransferredSupervisorLocksForTest(svc Service, ctx context.Context, groupIDs []int64, lockedStaff map[int64]bool) error {
	return svc.(*service).validateTransferredSupervisorLocks(ctx, groupIDs, lockedStaff)
}

// AssignSessionSupervisorsForTest runs the session start's best-effort
// supervisor assignment inside the caller's transaction.
func AssignSessionSupervisorsForTest(svc Service, ctx context.Context, groupID int64, staffIDs []int64, start time.Time) {
	svc.(*service).assignMultipleSupervisorsNonCritical(ctx, groupID, staffIDs, start)
}
