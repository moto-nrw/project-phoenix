package application

import (
	context "context"

	domain "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

type RoomDirectory struct{ Rooms facilitiesModule.Query }

func (d RoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]domain.Room, error) {
	rooms, err := d.Rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return ToDomainRooms(rooms), nil
}

func (d RoomDirectory) ListRooms(ctx context.Context) ([]domain.Room, error) {
	rooms, err := d.Rooms.ListRooms(ctx, facilitiesModule.RoomFilter{})
	if err != nil {
		return nil, err
	}
	return ToDomainRooms(rooms), nil
}

func ToDomainRooms(rooms []facilitiesModule.Room) []domain.Room {
	result := make([]domain.Room, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, domain.Room{
			ID: room.ID, TenantID: room.TenantID, Name: room.Name, Capacity: room.Capacity,
		})
	}
	return result
}
