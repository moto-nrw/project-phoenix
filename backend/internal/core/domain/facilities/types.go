package facilities

import "time"

// RoomHistoryEntry represents a single room history entry with student visit information
type RoomHistoryEntry struct {
	StudentID   int64      `json:"student_id" bun:"student_id"`
	StudentName string     `json:"student_name" bun:"student_name"`
	GroupID     int64      `json:"group_id" bun:"group_id"`
	GroupName   string     `json:"group_name" bun:"group_name"`
	CheckedIn   time.Time  `json:"checked_in" bun:"checked_in"`
	CheckedOut  *time.Time `json:"checked_out,omitempty" bun:"checked_out"`
}

// RoomWithOccupancy represents a room with its current occupancy status
type RoomWithOccupancy struct {
	*Room
	IsOccupied   bool    `json:"is_occupied" bun:"is_occupied"`
	GroupName    *string `json:"group_name,omitempty" bun:"group_name"`
	CategoryName *string `json:"category_name,omitempty" bun:"category_name"`
}
