package ports

type RoomOccupancy struct {
	RoomID             int64
	ActivityGroupIDs   []int64
	StudentCount       int
	SupervisorStaffIDs []int64
}
