package facilities

import (
	"context"
)

// RoomRepository defines the interface for room repository operations
type RoomRepository interface {
	// Create inserts a new room into the database
	Create(ctx context.Context, room *Room) error

	// FindByID retrieves a room by its ID
	FindByID(ctx context.Context, id interface{}) (*Room, error)

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
	Delete(ctx context.Context, id interface{}) error

	// List retrieves rooms matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Room, error)
}
