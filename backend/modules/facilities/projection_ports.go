package facilities

import (
	"context"
	"time"
)

type ActivityGroupName struct {
	Name     string
	Category *string
}

type ActivityGroupNames interface {
	ListActivityGroupNames(ctx context.Context, ids []int64) (map[int64]ActivityGroupName, error)
}

type RoomOccupancyFact struct {
	RoomID             int64
	ActivityGroupIDs   []int64
	StudentCount       int
	SupervisorStaffIDs []int64
}

type RoomOccupancyQuery interface {
	ListRoomOccupancy(context.Context, []int64) ([]RoomOccupancyFact, error)
}

type RoomSessionFact struct {
	SessionID          int64
	ActivityGroupID    *int64
	StartedAt          time.Time
	EndedAt            *time.Time
	DurationMinutes    *int
	SupervisorStaffIDs []int64
	StudentCount       int
}

type RoomHistoryQuery interface {
	ListRoomSessionHistory(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionFact, error)
}

type RoomRows interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*Room, error)
}

type RoomName struct {
	ID   int64
	Name string
}

type RoomNameDirectory interface {
	AllRoomNames(context.Context) (map[int64]string, error)
	RoomName(context.Context, int64) (string, bool, error)
	RoomNamesByID(context.Context, []int64) (map[int64]string, error)
	LockRoom(context.Context, int64) (*int, bool, error)
}

type SettingHandler func(context.Context, int64, any) (func(), error)
type SchulhofProvisioner interface {
	EnsureInfrastructure(context.Context, int64) (*SystemActivity, error)
}
type WCProvisioner interface {
	EnsureInfrastructure(context.Context) (*SystemActivity, error)
}
