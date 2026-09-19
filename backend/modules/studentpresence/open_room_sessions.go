package studentpresence

import (
	"context"
	"time"
)

type OpenRoomSession struct {
	ActiveGroupID      int64
	RoomID             int64
	ActivityGroupID    *int64
	DeviceID           *int64
	StartTime          time.Time
	SupervisorStaffIDs []int64
}

type OpenRoomSessionQuery interface {
	ListOpenRoomSessions(context.Context, []int64) ([]OpenRoomSession, error)
}

func (m *Module) ListOpenRoomSessions(ctx context.Context, ids []int64) ([]OpenRoomSession, error) {
	return m.engine.ListOpenRoomSessions(ctx, ids)
}
