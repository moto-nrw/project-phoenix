package repositories

import (
	"context"

	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// EducationRooms serves the group service's room directory from the
// Facilities rooms.
type EducationRooms struct{ rooms facilityModels.RoomRepository }

// NewEducationRooms binds the room directory to the room repository.
func NewEducationRooms(rooms facilityModels.RoomRepository) EducationRooms {
	return EducationRooms{rooms: rooms}
}

// FindRoom returns the room a group may be assigned to.
func (r EducationRooms) FindRoom(ctx context.Context, id int64) (*educationRepo.RoomProjection, error) {
	room, err := r.rooms.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &educationRepo.RoomProjection{
		ID: room.ID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
		Name: room.Name, Building: room.Building, Floor: room.Floor,
		Capacity: room.Capacity, Category: room.Category, Color: room.Color,
	}, nil
}
