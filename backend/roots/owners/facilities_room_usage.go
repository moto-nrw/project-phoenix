package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// RoomUsage binds the Facilities room usage port (#4010): Student Presence
// answers whether a session runs in the room, Care Plan whether a care
// offering needs it. Facilities decides what a deletion makes of the answers.
// The serving root binds both owners once the service graph exists; until
// then the deletion is refused.
type RoomUsage struct {
	Sessions interface {
		QueryLiveGroups(ctx context.Context, filter studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	}
	Offerings interface {
		ValidateRoomDeletion(ctx context.Context, roomID int64) error
	}
}

func (u *RoomUsage) HasActiveSessions(ctx context.Context, roomID int64) (bool, error) {
	if u.Sessions == nil {
		return false, facilitiesModule.ErrRoomDeletionGuardUnavailable
	}
	sessions, err := u.Sessions.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{RoomID: &roomID, OpenOnly: true})
	if err != nil {
		return false, err
	}
	return len(sessions) > 0, nil
}

func (u *RoomUsage) RequiredByCareOffering(ctx context.Context, roomID int64) (bool, error) {
	if u.Offerings == nil {
		return false, facilitiesModule.ErrRoomDeletionGuardUnavailable
	}
	err := u.Offerings.ValidateRoomDeletion(ctx, roomID)
	if errors.Is(err, careplan.ErrCareOfferingConfigInvalid) {
		return true, nil
	}
	return false, err
}
