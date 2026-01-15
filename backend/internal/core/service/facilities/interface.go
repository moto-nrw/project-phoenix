// backend/services/facilities/interface.go
package facilities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
)

// RoomCRUD handles basic room CRUD operations
type RoomCRUD interface {
	GetRoom(ctx context.Context, id int64) (*facilities.Room, error)
	GetRoomWithOccupancy(ctx context.Context, id int64) (*facilities.RoomWithOccupancy, error)
	CreateRoom(ctx context.Context, room *facilities.Room) error
	UpdateRoom(ctx context.Context, room *facilities.Room) error
	DeleteRoom(ctx context.Context, id int64) error
	ListRooms(ctx context.Context, options *base.QueryOptions) ([]facilities.RoomWithOccupancy, error)
}

// RoomFinder handles room lookup operations
type RoomFinder interface {
	FindRoomByName(ctx context.Context, name string) (*facilities.Room, error)
	FindRoomsByBuilding(ctx context.Context, building string) ([]*facilities.Room, error)
	FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error)
	FindRoomsByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error)
	GetRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*facilities.Room, error)
}

// RoomAvailability handles room availability and capacity operations
type RoomAvailability interface {
	CheckRoomAvailability(ctx context.Context, roomID int64, requiredCapacity int) (bool, error)
	GetAvailableRooms(ctx context.Context, capacity int) ([]*facilities.Room, error)
	GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]facilities.RoomWithOccupancy, error)
}

// RoomAnalytics handles room utilization and history operations
type RoomAnalytics interface {
	GetRoomUtilization(ctx context.Context, roomID int64) (float64, error)
	GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time) ([]RoomHistoryEntry, error)
}

// RoomMetadata handles building and category metadata
type RoomMetadata interface {
	GetBuildingList(ctx context.Context) ([]string, error)
	GetCategoryList(ctx context.Context) ([]string, error)
}

// Service composes all facilities-related operations.
// Existing callers can continue using this full interface.
// New code can depend on smaller sub-interfaces for better decoupling.
type Service interface {
	base.TransactionalService
	RoomCRUD
	RoomFinder
	RoomAvailability
	RoomAnalytics
	RoomMetadata
}

// RoomHistoryEntry is an alias to the model type for backward compatibility
type RoomHistoryEntry = facilities.RoomHistoryEntry
