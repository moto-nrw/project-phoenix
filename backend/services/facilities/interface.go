// backend/services/facilities/interface.go
package facilities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// Service defines operations for managing facilities
type Service interface {
	// Room operations
	GetRoom(ctx context.Context, id int64) (*facilities.Room, error)
	CreateRoom(ctx context.Context, room *facilities.Room) error
	UpdateRoom(ctx context.Context, room *facilities.Room) error
	DeleteRoom(ctx context.Context, id int64) error
	ListRooms(ctx context.Context, options *base.QueryOptions) ([]*facilities.Room, error)

	// Room search operations
	FindRoomByName(ctx context.Context, name string) (*facilities.Room, error)
	FindRoomsByBuilding(ctx context.Context, building string) ([]*facilities.Room, error)
	FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error)
	FindRoomsByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error)
	FindRoomsWithCapacity(ctx context.Context, minCapacity int) ([]*facilities.Room, error)
	SearchRoomsByText(ctx context.Context, searchText string) ([]*facilities.Room, error)
}
