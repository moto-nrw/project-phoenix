package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
)

// OpenRoomDirectory binds the shared-room projection to the public Facilities
// capability. Facilities owns the release decision.
type OpenRoomDirectory struct{ Rooms facilities.Query }

func (d OpenRoomDirectory) Released(ctx context.Context) ([]supervisiondashboard.ReleasedRoom, error) {
	released := true
	rooms, err := d.Rooms.ListRooms(ctx, facilities.RoomFilter{IsOpenRoom: &released})
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.ReleasedRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, supervisiondashboard.ReleasedRoom{ID: room.ID, Name: room.Name})
	}
	return result, nil
}
