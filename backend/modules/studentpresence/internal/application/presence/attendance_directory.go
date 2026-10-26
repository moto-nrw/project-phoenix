package presence

import (
	"context"
)

// AttendanceEducationGroups supplies the group-to-room directory for dashboards.
type AttendanceEducationGroups interface {
	ListGroupRooms(context.Context) ([]*EducationGroupRoom, error)
	StudentGroupIDs(context.Context, []int64) (map[int64]int64, error)
	StudentGroupID(context.Context, int64) (*int64, error)
}

type EducationGroupRoom struct {
	ID     int64
	RoomID *int64
}

// AttendanceStaffNames supplies the names attached to attendance records.
// StaffNames maps staff IDs to display names in one read within the school
// in context. Staff who left stay named; a member whose personal data the
// automatic deletion erased, or who is not found, is left out (#4761).
type AttendanceStaffNames interface {
	StaffNames(context.Context, []int64) (map[int64]string, error)
}
