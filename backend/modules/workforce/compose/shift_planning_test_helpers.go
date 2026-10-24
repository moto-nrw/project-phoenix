//go:build testhooks

package compose

import "github.com/moto-nrw/project-phoenix/modules/workforce"

// NewStaffScheduleOverview builds suites' week-grid reader; production uses NewShiftPlanning.
// Its public values use string dates and ClockLayout wall clocks, not row models.
func NewStaffScheduleOverview(deps StaffScheduleOverviewDependencies) workforce.StaffScheduleOverviewQuery {
	query, _ := newStaffScheduleOverview(deps)
	return query
}
