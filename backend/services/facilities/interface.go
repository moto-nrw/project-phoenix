// backend/services/facilities/interface.go
package facilities

import (
	"context"

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
}
