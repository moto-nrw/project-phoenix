package checkin

import (
	"context"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
)

// schulhofSpace configures the shared system-space bootstrap for the
// Schulhof area. The lookup deliberately swallows all errors and falls
// through to auto-create (the historical behavior of this path).
var schulhofSpace = systemSpace{
	label: "Schulhof",
	findRoom: func(ctx context.Context, rs *Resource) (*facilities.Room, error) {
		room, err := rs.FacilityService.FindRoomByName(ctx, constants.SchulhofRoomName)
		if err != nil || room == nil {
			return nil, nil
		}
		return room, nil
	},
	roomName:        constants.SchulhofRoomName,
	roomCapacity:    constants.SchulhofRoomCapacity,
	categoryName:    constants.SchulhofCategoryName,
	categoryDesc:    constants.SchulhofCategoryDescription,
	color:           constants.SchulhofColor,
	activityName:    constants.SchulhofActivityName,
	maxParticipants: constants.SchulhofMaxParticipants,
}

// ensureSchulhofRoom finds or creates the Schulhof room
func (rs *Resource) ensureSchulhofRoom(ctx context.Context) (*facilities.Room, error) {
	return rs.ensureSystemRoom(ctx, schulhofSpace)
}

// ensureSchulhofCategory finds or creates the Schulhof activity category
func (rs *Resource) ensureSchulhofCategory(ctx context.Context) (*activities.Category, error) {
	return rs.ensureSystemCategory(ctx, schulhofSpace)
}

// schulhofActivityGroup finds or creates the permanent Schulhof activity
// group, lazily auto-creating the Schulhof infrastructure (room, category,
// activity) on first use.
func (rs *Resource) schulhofActivityGroup(ctx context.Context) (*activities.Group, error) {
	return rs.systemActivityGroup(ctx, schulhofSpace)
}
