package facilities

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/constants"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
)

// FindCanonicalSchulhofRoom resolves the reserved Schulhof room without
// accepting a case-insensitive legacy collision or an unprotected normal room.
// FindRoomByName is intentionally case-insensitive, so every Schulhof bootstrap
// path must pass through this validation before adopting its result.
func FindCanonicalSchulhofRoom(ctx context.Context, facilityService Service) (*facilityModels.Room, error) {
	if facilityService == nil {
		return nil, errors.New("cannot find Schulhof room: facility service is nil")
	}

	room, err := facilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errors.New("cannot find Schulhof room: repository returned nil room")
	}
	if room.Name != constants.SchulhofRoomName {
		return nil, fmt.Errorf(
			"non-canonical room name %q conflicts with reserved name %q",
			room.Name,
			constants.SchulhofRoomName,
		)
	}
	if !room.IsSystem {
		return nil, fmt.Errorf(
			"reserved room %q is not marked as a system room",
			constants.SchulhofRoomName,
		)
	}
	return room, nil
}

// ValidateSchulhofActivityRoom verifies that the reserved activity is the
// system activity attached to the canonical Schulhof room.
func ValidateSchulhofActivityRoom(activityGroup *activityModels.Group, room *facilityModels.Room) error {
	if activityGroup == nil {
		return errors.New("schulhof activity is nil")
	}
	if room == nil {
		return errors.New("schulhof room is nil")
	}
	if activityGroup.Name != constants.SchulhofActivityName {
		return fmt.Errorf(
			"activity name %q does not match reserved name %q",
			activityGroup.Name,
			constants.SchulhofActivityName,
		)
	}
	if !activityGroup.IsSystem {
		return fmt.Errorf(
			"reserved activity %q is not marked as a system activity",
			constants.SchulhofActivityName,
		)
	}
	if activityGroup.PlannedRoomID == nil || *activityGroup.PlannedRoomID != room.ID {
		return fmt.Errorf(
			"reserved activity %q is not assigned to canonical Schulhof room %d",
			constants.SchulhofActivityName,
			room.ID,
		)
	}

	return nil
}
