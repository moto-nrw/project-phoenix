package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// RoomAvailabilitySource reads the existing tenant-safe Facilities projection.
type RoomAvailabilitySource interface {
	GetAvailableRoomsWithOccupancy(context.Context, int) ([]facilities.RoomWithOccupancy, error)
}

type roomAvailability struct {
	rooms      RoomAvailabilitySource
	principals ports.Principals
}

func NewRoomAvailability(rooms RoomAvailabilitySource, principals ports.Principals) devicescan.RoomAvailability {
	if rooms == nil || principals == nil {
		panic("kiosk room availability: rooms and principals are required")
	}
	return &roomAvailability{rooms: rooms, principals: principals}
}

func (q *roomAvailability) AvailableRooms(ctx context.Context, capacity int) ([]devicescan.AvailableRoom, error) {
	device, ok := q.principals.Device(ctx)
	if !ok || device == nil {
		return nil, devicescan.ErrDeviceUnauthorized
	}
	rooms, err := q.rooms.GetAvailableRoomsWithOccupancy(ctx, capacity)
	if err != nil {
		return nil, err
	}
	result := make([]devicescan.AvailableRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, devicescan.AvailableRoom{
			ID: room.ID, Name: room.Name, Building: room.Building,
			Floor: room.Floor, Capacity: room.Capacity, Category: room.Category,
			Color: room.Color, IsOccupied: room.IsOccupied,
		})
	}
	return result, nil
}
