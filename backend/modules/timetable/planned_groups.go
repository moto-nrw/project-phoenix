package timetable

import "context"

// PlannedGroupQuery answers which activity groups a staff member is planned to
// supervise.
type PlannedGroupQuery interface {
	ListPlannedGroupsForStaff(context.Context, int64) ([]Group, error)
}
