package active

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
type AttendanceStaffNames interface {
	StaffName(context.Context, int64) (string, error)
}
