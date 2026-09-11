package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
)

// openRoomDirectory binds the shared-room projection to the public Facilities
// capability. Facilities owns the release decision.
type openRoomDirectory struct{ rooms facilities.Query }

func (d openRoomDirectory) Released(ctx context.Context) ([]supervisiondashboard.ReleasedRoom, error) {
	released := true
	rooms, err := d.rooms.ListRooms(ctx, facilities.RoomFilter{IsOpenRoom: &released})
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.ReleasedRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, supervisiondashboard.ReleasedRoom{ID: room.ID, Name: room.Name})
	}
	return result, nil
}
