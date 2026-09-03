package users

import (
	"context"
	"errors"
)

// DirectoryRoom is the Facilities projection this package reads.
// facilities.rooms belongs to that owner (#2665); the composition root binds
// the directory behind RoomDirectory instead of the former SQL subquery.
type DirectoryRoom struct {
	ID       int64
	TenantID int64
}

// RoomDirectory is the owner query the care-exit restore validates room
// references through. It fails while unbound; there is no fallback join.
type RoomDirectory interface {
	// LockRoomsByID returns the rooms visible in the caller's transaction and
	// keeps them from deletion until that transaction commits or rolls back.
	// Missing IDs are absent.
	LockRoomsByID(ctx context.Context, ids []int64) ([]DirectoryRoom, error)
}

var errRoomDirectoryRequired = errors.New("users repositories: room directory is not bound")

// validRoomIDs returns the subset of ids that the owner still resolves to a
// room of the given tenant. The restore replays only those references; a
// room deleted in the meantime restores as NULL, as the former subquery did.
func validRoomIDs(ctx context.Context, directory RoomDirectory, tenantID int64, ids []int64) ([]int64, error) {
	if directory == nil {
		return nil, errRoomDirectoryRequired
	}
	valid := []int64{}
	candidates := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return valid, nil
	}
	rooms, err := directory.LockRoomsByID(ctx, candidates)
	if err != nil {
		return nil, err
	}
	for _, room := range rooms {
		if room.TenantID == tenantID {
			valid = append(valid, room.ID)
		}
	}
	return valid, nil
}
