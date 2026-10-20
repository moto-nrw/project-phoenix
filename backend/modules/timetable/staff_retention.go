package timetable

import "context"

// RetiredStaffAssignments reports the timetable rows erased for a retired
// staff member after the retention period.
type RetiredStaffAssignments struct {
	PlannedSupervisors int64
	InstanceStaff      int64
}

// StaffRetentionCapability removes every personal timetable assignment of a
// retired staff member. The caller owns the surrounding tenant transaction.
type StaffRetentionCapability interface {
	DeleteRetiredStaffAssignments(context.Context, int64) (RetiredStaffAssignments, error)
}
