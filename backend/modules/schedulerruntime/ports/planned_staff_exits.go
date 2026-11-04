package ports

import "context"

// PlannedStaffExits sends the reminders of the planned staff exits of the
// school in ctx and offboards the members whose last working day is over
// (#4809). A blocked offboarding closes the member's sign-in and is retried
// hourly. The root binds the Staff Offboarding workflow's planner.
type PlannedStaffExits func(ctx context.Context) error
