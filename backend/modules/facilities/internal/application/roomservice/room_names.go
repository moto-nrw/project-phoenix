package facilities

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// RoomNameDirectory names and locks the retained Facilities rooms for the
// Timetable owner's operational day and instance lifecycle, which may not
// name the room rows themselves (#3551).
type RoomNameDirectory struct {
	rooms RoomNameRecords
}

// NewRoomNameDirectory binds the directory to the retained room records.
func NewRoomNameDirectory(rooms RoomNameRecords) RoomNameDirectory {
	return RoomNameDirectory{rooms: rooms}
}

// AllRoomNames names every room of the tenant.
func (r RoomNameDirectory) AllRoomNames(ctx context.Context) (map[int64]string, error) {
	rooms, err := r.rooms.List(ctx, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		if room != nil {
			names[room.ID] = room.Name
		}
	}
	return names, nil
}

// RoomName names one room; ok is false when it does not exist.
func (r RoomNameDirectory) RoomName(ctx context.Context, id int64) (string, bool, error) {
	room, err := r.rooms.FindByID(ctx, id)
	if err != nil || room == nil {
		return "", false, err
	}
	return room.Name, true, nil
}

// RoomNamesByID names the given rooms; a missing room is absent.
func (r RoomNameDirectory) RoomNamesByID(ctx context.Context, ids []int64) (map[int64]string, error) {
	rooms, err := r.rooms.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		if room != nil {
			names[room.ID] = room.Name
		}
	}
	return names, nil
}

// LockRoom takes the room's row lock. ok is false when the room does not
// exist; a nil capacity is unlimited.
func (r RoomNameDirectory) LockRoom(ctx context.Context, id int64) (*int, bool, error) {
	room, err := r.rooms.FindByIDForUpdate(ctx, id)
	if err != nil || room == nil {
		return nil, false, err
	}
	return room.Capacity, true, nil
}

type RoomNameRecords interface {
	List(context.Context, map[string]any) ([]*facilitiesModule.Room, error)
	FindByID(context.Context, any) (*facilitiesModule.Room, error)
	FindByIDs(context.Context, []int64) ([]*facilitiesModule.Room, error)
	FindByIDForUpdate(context.Context, int64) (*facilitiesModule.Room, error)
}
