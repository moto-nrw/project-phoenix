package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentLifecycleRepository is the narrow contract the activate-students tick
// needs from the student repository. The root binds the retained repository;
// statuses travel as their stored values.
type StudentLifecycleRepository interface {
	// FindPendingDueForActivation returns the pending children whose care
	// starts on or before asOf.
	FindPendingDueForActivation(ctx context.Context, asOf calendar.Date) ([]int64, error)
	// FindActiveDueForDeactivation returns the active children whose care
	// ended before asOf.
	FindActiveDueForDeactivation(ctx context.Context, asOf calendar.Date) ([]int64, error)
	// Compare-and-set, deliberately not the unconditional UpdateStatus: the tick
	// decides from rows it read earlier, and by the time it writes, a grade
	// transition may have graduated the child (see the repository method's doc).
	TransitionStatus(ctx context.Context, studentID int64, expected, next string) (bool, error)
}

// CareExitEffector performs the effect-day housekeeping for children whose
// care has ended (#2487): closing what is still open, freeing the bracelet,
// closing the open parent requests. Narrow contract so the scheduler does not
// import the whole users service package.
type CareExitEffector interface {
	ApplyDueEffects(ctx context.Context, asOf calendar.Date) (int, error)
}

// StudentLifecycleAuditor records scheduler-authored status transitions.
type StudentLifecycleAuditor interface {
	RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error
}
