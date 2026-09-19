package ports

import "time"

type OpenRoomSession struct {
	ActiveGroupID      int64
	RoomID             int64
	ActivityGroupID    *int64
	DeviceID           *int64
	StartTime          time.Time
	SupervisorStaffIDs []int64
}
