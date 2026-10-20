package studentpresence

import (
	"context"
	"time"
)

type RoomSessionHistory struct {
	SessionID          int64
	ActivityGroupID    *int64
	StartedAt          time.Time
	EndedAt            *time.Time
	DurationMinutes    *int
	SupervisorStaffIDs []int64
	StudentCount       int
}

type RoomHistoryQuery interface {
	ListRoomSessionHistory(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionHistory, error)
}
