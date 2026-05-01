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
	// Room operations
	GetRoom(ctx context.Context, id int64) (*facilities.Room, error)
	GetRoomWithOccupancy(ctx context.Context, id int64) (RoomWithOccupancy, error)
	CreateRoom(ctx context.Context, room *facilities.Room) error
	UpdateRoom(ctx context.Context, room *facilities.Room) error
	DeleteRoom(ctx context.Context, id int64) error
	ListRooms(ctx context.Context, options *base.QueryOptions) ([]RoomWithOccupancy, error)
	FindRoomByName(ctx context.Context, name string) (*facilities.Room, error)
	// FindToiletRoom returns the first existing toilet special room whose
	// name matches a canonical alias (WC, then Toilette) case-insensitively,
	// preferring earlier entries. Returns ErrRoomNotFound (wrapped in
	// FacilitiesError) when no alias exists.
	//
	// excludeRoomID > 0 skips the row with that ID and continues searching
	// the remaining aliases — used by update-time alias-collision checks
	// where the row currently being updated must not count as its own
	// duplicate. It does NOT short-circuit the loop, so a tenant with both
	// aliases will still surface the non-excluded one.
	FindToiletRoom(ctx context.Context, excludeRoomID int64) (*facilities.Room, error)
	FindRoomsByBuilding(ctx context.Context, building string) ([]*facilities.Room, error)
	FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error)
	FindRoomsByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error)

	// Advanced operations
	CheckRoomAvailability(ctx context.Context, roomID int64, requiredCapacity int) (bool, error)
	GetAvailableRooms(ctx context.Context, capacity int) ([]*facilities.Room, error)
	GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]RoomWithOccupancy, error)
	GetRoomUtilization(ctx context.Context, roomID int64) (float64, error)
	GetBuildingList(ctx context.Context) ([]string, error)
	GetCategoryList(ctx context.Context) ([]string, error)
	GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time) ([]RoomHistoryEntry, error)
}

// RoomWithOccupancy represents a room with its current occupancy status
type RoomWithOccupancy struct {
	*facilities.Room
	IsOccupied      bool    `json:"is_occupied"`
	GroupName       *string `json:"group_name,omitempty"`
	CategoryName    *string `json:"category_name,omitempty"`
	StudentCount    int     `json:"student_count"`
	SupervisorNames *string `json:"supervisor_names,omitempty"`
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
