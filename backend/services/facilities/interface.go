package facilities

import (
	"context"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Service is the compatibility surface for callers not yet converted to the
// owner capability. Room persistence always goes through Facilities.
type Service interface {
	GetRoom(context.Context, int64) (*facilitiesModule.Room, error)
	GetRoomWithOccupancy(context.Context, int64) (RoomWithOccupancy, error)
	CreateRoom(context.Context, *facilitiesModule.Room) error
	UpdateRoom(context.Context, *facilitiesModule.Room) error
	DeleteRoom(context.Context, int64) error
	ValidateRoomDeletion(context.Context, int64) error
	ListRooms(context.Context) ([]RoomWithOccupancy, error)
	ProjectRoomOccupancy(context.Context, []facilitiesModule.Room) ([]RoomWithOccupancy, error)
	FindRoomByName(context.Context, string) (*facilitiesModule.Room, error)
	FindToiletRoom(context.Context, int64) (*facilitiesModule.Room, error)
	FindRoomsByCategory(context.Context, string) ([]*facilitiesModule.Room, error)
	GetAvailableRooms(context.Context, int) ([]*facilitiesModule.Room, error)
	GetAvailableRoomsWithOccupancy(context.Context, int) ([]RoomWithOccupancy, error)
	GetBuildingList(context.Context) ([]string, error)
	GetCategoryList(context.Context) ([]string, error)
	GetRoomHistory(context.Context, int64, time.Time, time.Time, *int64) ([]RoomSessionEntry, error)
}

type RoomWithOccupancy struct {
	*facilitiesModule.Room
	IsOccupied      bool    `json:"is_occupied"`
	GroupName       *string `json:"group_name,omitempty"`
	CategoryName    *string `json:"category_name,omitempty"`
	StudentCount    int     `json:"student_count"`
	SupervisorNames *string `json:"supervisor_names,omitempty"`
}

type RoomSessionEntry struct {
	SessionID       int64      `json:"session_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	ActivityName    string     `json:"activity_name"`
	SupervisorName  string     `json:"supervisor_name"`
	StudentCount    int        `json:"student_count"`
}
