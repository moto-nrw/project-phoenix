package facilities

import (
	"context"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Service is the compatibility surface for callers not yet converted to the
// owner capability. Room persistence always goes through Facilities.
type Service interface {
	ListRooms(context.Context) ([]RoomWithOccupancy, error)
	GetRoomWithOccupancy(context.Context, int64) (RoomWithOccupancy, error)
	CreateRoom(context.Context, *facilitiesModule.Room) error
	UpdateRoom(context.Context, *facilitiesModule.Room) error
	ProjectRoomOccupancy(context.Context, []facilitiesModule.Room) ([]RoomWithOccupancy, error)
	FindRoomByName(context.Context, string) (*facilitiesModule.Room, error)
	FindToiletRoom(context.Context, int64) (*facilitiesModule.Room, error)
	GetAvailableRoomsWithOccupancy(context.Context, int) ([]RoomWithOccupancy, error)
	GetBuildingList(context.Context) ([]string, error)
	GetCategoryList(context.Context) ([]string, error)
	GetRoomHistory(context.Context, int64, time.Time, time.Time, *int64) ([]facilitiesModule.RoomSessionEntry, error)
}

type RoomWithOccupancy = facilitiesModule.RoomWithOccupancy
