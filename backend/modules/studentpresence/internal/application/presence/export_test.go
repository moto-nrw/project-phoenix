package presence

import "context"

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
