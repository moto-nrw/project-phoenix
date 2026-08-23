package checkin

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
)

// wcSpace configures the shared system-space bootstrap for the WC area.
// The lookup goes through FindToiletRoom (accepts room-name aliases) and —
// unlike Schulhof — aborts on unexpected lookup errors instead of falling
// through to auto-create, so alias collisions surface instead of spawning
// duplicate rooms.
var wcSpace = systemSpace{
	label:       "WC",
	logRoomName: true,
	findRoom: func(ctx context.Context, s *CheckinService) (*facilities.Room, error) {
		room, err := s.facilities.FindToiletRoom(ctx, 0)
		if err != nil {
			if errors.Is(err, facilitiesSvc.ErrRoomNotFound) {
				return nil, nil
			}
			return nil, fmt.Errorf("failed to look up WC room: %w", err)
		}
		return room, nil
	},
	roomName:        constants.WCRoomName,
	roomCapacity:    constants.WCRoomCapacity,
	categoryName:    constants.WCCategoryName,
	categoryDesc:    constants.WCCategoryDescription,
	color:           constants.WCColor,
	activityName:    constants.WCActivityName,
	maxParticipants: constants.WCMaxParticipants,
}

// wcActivityGroup finds or creates the permanent WC activity group, lazily
// auto-creating the WC infrastructure (room, category, activity) on first use.
func (s *CheckinService) wcActivityGroup(ctx context.Context) (*activities.Group, error) {
	return s.systemActivityGroup(ctx, wcSpace)
}
