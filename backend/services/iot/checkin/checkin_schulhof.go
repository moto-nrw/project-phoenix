package checkin

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/activities"
	facilityModels "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
)

// schulhofSpace configures the shared system-space bootstrap for the Schulhof
// area. Its case-insensitive repository lookup is validated before any existing
// room can be adopted as reserved infrastructure.
var schulhofSpace = systemSpace{
	label: "Schulhof",
	findRoom: func(ctx context.Context, s *CheckinService) (*facilityModels.Room, error) {
		room, err := facilitiesSvc.FindCanonicalSchulhofRoom(ctx, s.facilities)
		if errors.Is(err, facilitiesSvc.ErrRoomNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("failed to look up Schulhof room: %w", err)
		}
		return room, nil
	},
	roomName:        constants.SchulhofRoomName,
	roomCapacity:    constants.SchulhofRoomCapacity,
	categoryName:    constants.SchulhofCategoryName,
	categoryDesc:    constants.SchulhofCategoryDescription,
	color:           constants.SchulhofColor,
	roomColorless:   true,
	activityName:    constants.SchulhofActivityName,
	maxParticipants: constants.SchulhofMaxParticipants,
	selectActivity: func(groups []*activities.Group, room *facilityModels.Room) *activities.Group {
		for _, group := range groups {
			if facilitiesSvc.ValidateSchulhofActivityRoom(systemActivity(group), room) == nil {
				return group
			}
		}
		return nil
	},
}

// schulhofActivityGroup finds or creates the permanent Schulhof activity group,
// lazily auto-creating the Schulhof infrastructure (room, category, activity)
// on first use.
func (s *CheckinService) schulhofActivityGroup(ctx context.Context) (*activities.Group, error) {
	activityGroup, err := s.systemActivityGroup(ctx, schulhofSpace)
	if err != nil {
		return nil, err
	}
	room, err := facilitiesSvc.FindCanonicalSchulhofRoom(ctx, s.facilities)
	if err != nil {
		return nil, fmt.Errorf("failed to validate Schulhof room: %w", err)
	}
	if err := facilitiesSvc.ValidateSchulhofActivityRoom(systemActivity(activityGroup), room); err != nil {
		return nil, fmt.Errorf("invalid Schulhof activity infrastructure: %w", err)
	}

	return activityGroup, nil
}

func systemActivity(group *activities.Group) *facilitiesSvc.SystemActivity {
	if group == nil {
		return nil
	}
	return &facilitiesSvc.SystemActivity{
		ID: group.ID, Name: group.Name, PlannedRoomID: group.PlannedRoomID, IsSystem: group.IsSystem,
	}
}
