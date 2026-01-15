package facilities

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// RoomRepository defines the interface for room repository operations
type RoomRepository interface {
	// Create inserts a new room into the database
	Create(ctx context.Context, room *Room) error

	// FindByID retrieves a room by its ID
	FindByID(ctx context.Context, id any) (*Room, error)

	// FindByName retrieves a room by its name
	FindByName(ctx context.Context, name string) (*Room, error)

	// FindByBuilding retrieves rooms by building
	FindByBuilding(ctx context.Context, building string) ([]*Room, error)

	// FindByCategory retrieves rooms by category
	FindByCategory(ctx context.Context, category string) ([]*Room, error)

	// FindByFloor retrieves rooms by building and floor
	FindByFloor(ctx context.Context, building string, floor int) ([]*Room, error)

	// Update updates an existing room
	Update(ctx context.Context, room *Room) error

	// Delete removes a room
	Delete(ctx context.Context, id any) error

	// List retrieves rooms matching the filters
	List(ctx context.Context, filters map[string]any) ([]*Room, error)

	// FindByIDs retrieves rooms by their IDs
	FindByIDs(ctx context.Context, ids []int64) ([]*Room, error)

	// GetRoomHistory retrieves visit history for a room within the specified time range
	GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time) ([]RoomHistoryEntry, error)

	// FindByIDWithOccupancy retrieves a room by ID with its current occupancy status
	FindByIDWithOccupancy(ctx context.Context, id int64) (*RoomWithOccupancy, error)

	// ListWithOccupancy retrieves all rooms with their current occupancy status
	ListWithOccupancy(ctx context.Context, options *base.QueryOptions) ([]RoomWithOccupancy, error)
}
