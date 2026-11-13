package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
	educationRepo "github.com/moto-ogs/project-phoenix/modules/schoolstructure/compose"
)

// EducationRooms serves the group service's room directory from the
// Facilities rooms.
type EducationRooms struct{ rooms EducationRoomQuery }

// EducationRoomQuery is the Facilities room read the directory needs.
type EducationRoomQuery interface {
	FindRoom(ctx context.Context, id int64) (facilities.Room, error)
}

// NewEducationRooms binds the room directory to the Facilities capability.
func NewEducationRooms(rooms EducationRoomQuery) EducationRooms {
	return EducationRooms{rooms: rooms}
}

// FindRoom returns the room a group may be assigned to.
func (r EducationRooms) FindRoom(ctx context.Context, id int64) (*educationRepo.RoomProjection, error) {
	room, err := r.rooms.FindRoom(ctx, id)
	if err != nil {
		return nil, err
	}
	// An archived room only keeps its history; no group may move into it
	// (#6845).
	if room.ArchivedAt != nil {
		return nil, facilities.ErrRoomNotFound
	}
	return &educationRepo.RoomProjection{
		ID: room.ID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
		Name: room.Name, Building: room.Building, Floor: room.Floor,
		Capacity: room.Capacity, Category: room.Category, Color: room.Color,
	}, nil
}
