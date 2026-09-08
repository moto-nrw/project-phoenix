package services

// Adapters that bind the shared open-room view's consumer-owned ports to the
// real owners (#3065). They live at the composition root on purpose: the
// projection names what it needs, and this is where those names are satisfied,
// so the projection package never imports a foreign owner's repository.

import (
	"context"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/services/supervisiondashboard"
)

// openRoomDirectory answers "which rooms are released" through the Facilities
// capability. Facilities owns the release; this only reads it.
type openRoomDirectory struct{ rooms facilities.Query }

func (d openRoomDirectory) ListReleasedRooms(ctx context.Context) ([]supervisiondashboard.ReleasedRoom, error) {
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

// openRoomSessions answers which sessions run in those rooms, with the offering
// each is backed by and who supervises it.
type openRoomSessions struct{ groups activeModel.GroupRepository }

func (s openRoomSessions) ListRunningSessionsInRooms(
	ctx context.Context, roomIDs []int64,
) ([]supervisiondashboard.RunningSession, error) {
	sessions, err := s.groups.FindOpenSessionsInRooms(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.RunningSession, 0, len(sessions))
	for _, session := range sessions {
		// A session without a template runs without an offering. That is a
		// fact the view displays as such; it is never filled in with a
		// placeholder (#3062), so an empty name is carried through as is.
		result = append(result, supervisiondashboard.RunningSession{
			ActiveGroupID:      session.ActiveGroupID,
			RoomID:             session.RoomID,
			ActivityName:       session.ActivityName,
			StartTime:          session.StartTime,
			SupervisorStaffIDs: session.SupervisorStaffIDs,
		})
	}
	return result, nil
}
