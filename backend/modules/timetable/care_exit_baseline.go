package timetable

import "context"

// CareExitBaselineQuery answers the "Betreuung beenden" preview: how many
// planned roster rows and running activity bookings ending a child's care
// would take away. Both counts are measured against the plan BEFORE any exit:
// the rows and bookings an earlier, still-planned exit of the same children
// removed come back when that exit is changed, so the caller hands them in as
// restorable and they are counted with the live ones. A row or booking that
// was put back by hand since is live and counted once.
type CareExitBaselineQuery interface {
	// CountPlannedRosterForCareExit counts, per student, the roster rows
	// dated after `after` (YYYY-MM-DD) that are neither observed nor on a
	// completed or cancelled instance.
	CountPlannedRosterForCareExit(ctx context.Context, studentIDs []int64, after string, restorable []CareExitRosterRow) (map[int64]int, error)
	// CountRunningEnrollmentsForCareExit counts, per student, the activity
	// bookings still running on validUntil (YYYY-MM-DD, exclusive end).
	CountRunningEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string, restorable []CareExitEnrollmentRemoval) (map[int64]int, error)
}
