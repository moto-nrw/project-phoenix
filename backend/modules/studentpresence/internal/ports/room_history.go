package ports

import "time"

type RoomSessionHistory struct {
	SessionID          int64
	ActivityGroupID    *int64
	StartedAt          time.Time
	EndedAt            *time.Time
	DurationMinutes    *int
	SupervisorStaffIDs []int64
	StudentCount       int
}
