package active

import (
	"context"
	"errors"
	"time"

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
	IsSystem  bool
}

// RoomDirectory is the owner query the active repositories read rooms
// through. Every method fails while unbound; there is no fallback join.
type RoomDirectory interface {
	// ListRoomsByID returns the rooms visible in the caller's transaction.
	// Missing IDs are absent, like the former LEFT JOIN.
	ListRoomsByID(ctx context.Context, ids []int64) ([]DirectoryRoom, error)
}

var errRoomDirectoryRequired = errors.New("active repositories: room directory is not bound")

// roomsByID resolves the unique positive ids through the bound directory.
func roomsByID(ctx context.Context, directory RoomDirectory, ids []int64) (map[int64]DirectoryRoom, error) {
	if directory == nil {
		return nil, errRoomDirectoryRequired
	}
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	result := make(map[int64]DirectoryRoom, len(unique))
	if len(unique) == 0 {
		return result, nil
	}
	rooms, err := directory.ListRoomsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		result[room.ID] = room
	}
	return result, nil
}

// legacy rebuilds the full facilities.Room row the former batch load
// scanned, so group consumers (location snapshot colours included) see the
// same shape as before.
func (r DirectoryRoom) legacy() *facilities.Room {
	room := &facilities.Room{
		ID:        r.ID,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
		Name:      r.Name,
		Building:  r.Building,
		Floor:     r.Floor,
		Capacity:  r.Capacity,
		Category:  r.Category,
		Color:     r.Color,
		IsSystem:  r.IsSystem,
	}
	room.SetTenantID(r.TenantID)
	return room
}
