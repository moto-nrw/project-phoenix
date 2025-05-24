// backend/services/facilities/interface.go
package facilities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// Service defines operations for managing facilities
type Service interface {
	base.TransactionalService
	// Room operations
	GetRoom(ctx context.Context, id int64) (*facilities.Room, error)
	CreateRoom(ctx context.Context, room *facilities.Room) error
	UpdateRoom(ctx context.Context, room *facilities.Room) error
	DeleteRoom(ctx context.Context, id int64) error
	ListRooms(ctx context.Context, options *base.QueryOptions) ([]*facilities.Room, error)
	FindRoomByName(ctx context.Context, name string) (*facilities.Room, error)
	FindRoomsByBuilding(ctx context.Context, building string) ([]*facilities.Room, error)
	FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error)
	FindRoomsByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error)

	// Advanced operations
	CheckRoomAvailability(ctx context.Context, roomID int64, requiredCapacity int) (bool, error)
	GetAvailableRooms(ctx context.Context, capacity int) ([]*facilities.Room, error)
	GetRoomUtilization(ctx context.Context, roomID int64) (float64, error)
	GetBuildingList(ctx context.Context) ([]string, error)
	GetCategoryList(ctx context.Context) ([]string, error)
	GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time) ([]RoomHistoryEntry, error)
}

// RoomHistoryEntry represents a single room history entry
type RoomHistoryEntry struct {
	StudentID   int64      `json:"student_id"`
	StudentName string     `json:"student_name"`
	GroupID     int64      `json:"group_id"`
	GroupName   string     `json:"group_name"`
	CheckedIn   time.Time  `json:"checked_in"`
	CheckedOut  *time.Time `json:"checked_out,omitempty"`
}
