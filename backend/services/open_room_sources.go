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
	groups, err := s.groups.FindActiveByRoomIDs(ctx, roomIDs)
	if err != nil {
		return nil, err
	}
	result := make([]supervisiondashboard.RunningSession, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		session := supervisiondashboard.RunningSession{
			ActiveGroupID: group.ID,
			RoomID:        group.RoomID,
			StartTime:     group.StartTime,
		}
		// A session without a template runs without an offering. That is a
		// fact the view displays as such; it is never filled in with a
		// placeholder (#3062).
		if group.ActualGroup != nil {
			session.ActivityName = group.ActualGroup.Name
		}
		session.SupervisorStaffIDs = activeSupervisorStaffIDs(group.Supervisors)
		result = append(result, session)
	}
	return result, nil
}

// activeSupervisorStaffIDs keeps only supervisions that have not ended. A
// closed supervision must not make the room read as the caller's own.
func activeSupervisorStaffIDs(supervisors []*activeModel.GroupSupervisor) []int64 {
	ids := make([]int64, 0, len(supervisors))
	for _, supervisor := range supervisors {
		if supervisor == nil || supervisor.EndDate != nil {
			continue
		}
		ids = append(ids, supervisor.StaffID)
	}
	return ids
}
