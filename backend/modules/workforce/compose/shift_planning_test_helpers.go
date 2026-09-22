package compose

import "github.com/moto-nrw/project-phoenix/modules/workforce"

// NewStaffScheduleOverview composes the public week-grid read for suites that
// drive a retained consumer over their own readers; production binds the
// overview through NewShiftPlanning. The result carries string dates and
// ClockLayout wall clocks, so a consumer never touches the row models.
func NewStaffScheduleOverview(deps StaffScheduleOverviewDependencies) workforce.StaffScheduleOverviewQuery {
	query, _ := newStaffScheduleOverview(deps)
	return query
}
