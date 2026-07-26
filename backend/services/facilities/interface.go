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
	FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error)

	// Advanced operations
	GetAvailableRooms(ctx context.Context, capacity int) ([]*facilities.Room, error)
	GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]RoomWithOccupancy, error)
	GetBuildingList(ctx context.Context) ([]string, error)
	GetCategoryList(ctx context.Context) ([]string, error)
	// GetRoomHistory returns the aggregated session timeline for a room in
	// [startTime, endTime]. Each entry is one active.groups session — no
	// per-student rows leave this layer (see the matching frontend drawer
	// contract). When supervisorStaffID is non-nil the result is filtered
	// to sessions supervised by that staff member; pass nil to see every
	// session (admin or all_staff scope).
	GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time, supervisorStaffID *int64) ([]RoomSessionEntry, error)
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

// RoomSessionEntry is one aggregated session row for the room-history
// endpoint. Deliberately contains no per-student IDs or names — only the
// distinct student count plus the staff/activity metadata needed to render
// the timeline. Per-child movement detail is served separately by
// /students/{id}/attendance-history under its own GDPR gates.
type RoomSessionEntry struct {
	SessionID       int64      `json:"session_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	ActivityName    string     `json:"activity_name"`
	SupervisorName  string     `json:"supervisor_name"`
	StudentCount    int        `json:"student_count"`
}
