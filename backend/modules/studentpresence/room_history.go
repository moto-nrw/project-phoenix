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

func (m *Module) ListRoomSessionHistory(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]RoomSessionHistory, error) {
	return m.engine.ListRoomSessionHistory(ctx, roomID, start, end, staffID)
}
