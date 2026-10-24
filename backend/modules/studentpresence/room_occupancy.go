package studentpresence

import "context"

type RoomOccupancy struct {
	RoomID             int64
	ActivityGroupIDs   []int64
	StudentCount       int
	SupervisorStaffIDs []int64
}

type RoomOccupancyQuery interface {
	ListRoomOccupancy(context.Context, []int64) ([]RoomOccupancy, error)
}
