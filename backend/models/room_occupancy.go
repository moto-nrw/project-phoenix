package models

// RoomOccupancyDetail represents details of a room occupancy that are needed
// by the student visit tracking system
type RoomOccupancyDetail struct {
	RoomID     int64
	TimespanID int64
}
