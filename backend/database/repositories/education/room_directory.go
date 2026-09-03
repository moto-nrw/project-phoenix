package education

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// DirectoryRoom is the Facilities projection this package reads.
// facilities.rooms belongs to that owner (#2665); the composition root binds
// the directory behind RoomDirectory instead of the former SQL joins.
type DirectoryRoom struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Building  string
	Floor     *int
	Capacity  *int
	Category  *string
	Color     *string
}

// RoomDirectory is the owner query the education repositories read rooms
// through. Every method fails while unbound; there is no fallback join.
type RoomDirectory interface {
	// ListRoomsByID returns the rooms visible in the caller's transaction.
	// Missing IDs are absent, like the former LEFT JOIN.
	ListRoomsByID(ctx context.Context, ids []int64) ([]DirectoryRoom, error)
}

var errRoomDirectoryRequired = errors.New("education repositories: room directory is not bound")

// attachRooms fills Group.Room for every group with a visible room. A group
// whose room the owner cannot see keeps Room nil, as the LEFT JOIN did.
func attachRooms(ctx context.Context, directory RoomDirectory, groups []*education.Group) error {
	if directory == nil {
		return errRoomDirectoryRequired
	}
	ids := make([]int64, 0, len(groups))
	seen := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group == nil || group.RoomID == nil || *group.RoomID <= 0 {
			continue
		}
		if _, found := seen[*group.RoomID]; found {
			continue
		}
		seen[*group.RoomID] = struct{}{}
		ids = append(ids, *group.RoomID)
	}
	if len(ids) == 0 {
		return nil
	}
	rooms, err := directory.ListRoomsByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]DirectoryRoom, len(rooms))
	for _, room := range rooms {
		byID[room.ID] = room
	}
	for _, group := range groups {
		if group == nil || group.RoomID == nil {
			continue
		}
		if room, ok := byID[*group.RoomID]; ok {
			group.Room = room.legacy()
		}
	}
	return nil
}

func (r DirectoryRoom) legacy() *facilities.Room {
	return &facilities.Room{
		ID:        r.ID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		Name:      r.Name,
		Building:  r.Building,
		Floor:     r.Floor,
		Capacity:  r.Capacity,
		Category:  r.Category,
		Color:     r.Color,
	}
}
