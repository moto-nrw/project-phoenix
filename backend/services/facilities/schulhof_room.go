package facilities

import (
	"context"
	"errors"
	"fmt"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

func FindCanonicalSchulhofRoom(ctx context.Context, rooms Service) (*facilitiesModule.Room, error) {
	if rooms == nil {
		return nil, errors.New("cannot find Schulhof room: facility service is nil")
	}
	room, err := rooms.FindRoomByName(ctx, facilitiesModule.SchulhofRoomName)
	if err != nil {
		return nil, err
	}
	if room == nil {
		return nil, errors.New("cannot find Schulhof room: owner returned nil room")
	}
	if room.Name != facilitiesModule.SchulhofRoomName {
		return nil, fmt.Errorf("non-canonical room name %q conflicts with reserved name %q", room.Name, facilitiesModule.SchulhofRoomName)
	}
	if !room.IsSystem {
		return nil, fmt.Errorf("reserved room %q is not marked as a system room", facilitiesModule.SchulhofRoomName)
	}
	return room, nil
}

func ValidateSchulhofActivityRoom(activity *SystemActivity, room *facilitiesModule.Room) error {
	if activity == nil {
		return errors.New("schulhof activity is nil")
	}
	if room == nil {
		return errors.New("schulhof room is nil")
	}
	if activity.Name != facilitiesModule.SchulhofActivityName {
		return fmt.Errorf("activity name %q does not match reserved name %q", activity.Name, facilitiesModule.SchulhofActivityName)
	}
	if !activity.IsSystem {
		return fmt.Errorf("reserved activity %q is not marked as a system activity", facilitiesModule.SchulhofActivityName)
	}
	if activity.PlannedRoomID == nil || *activity.PlannedRoomID != room.ID {
		return fmt.Errorf("reserved activity %q is not assigned to canonical Schulhof room %d", facilitiesModule.SchulhofActivityName, room.ID)
	}
	return nil
}
