package api

import (
	"fmt"
	"log/slog"

	movebinding "github.com/moto-ogs/project-phoenix/roots/openroommovebinding"

	facilitiesModule "github.com/moto-ogs/project-phoenix/modules/facilities"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/timetable"

	"github.com/moto-ogs/project-phoenix/workflows/openroommove"
	openRoomMoveCompose "github.com/moto-ogs/project-phoenix/workflows/openroommove/compose"
)

// newOpenRoomMove composes the open-room move workflow (#3066) over the owner
// facades the root already holds: Facilities for the release, Timetable &
// Activities for the room's system activity, and the retained Student
// Presence service for the room session and the move until that owner exposes
// them publicly.
func newOpenRoomMove(rooms facilitiesModule.Capability, activities timetable.Capability, presence movebinding.Presence, logger *slog.Logger) (openroommove.Command, error) {
	// The presence value is Student Presence's composed facade, which also
	// serves the check-in into a room session (#6953) as its own capability.
	checkIns, ok := presence.(studentpresence.SessionCheckIns)
	if !ok {
		return nil, fmt.Errorf("open room move: %T does not check children in to a room session", presence)
	}
	log := logger.With("workflow", "open-room-move")
	return openRoomMoveCompose.New(openRoomMoveCompose.Dependencies{
		Rooms:        rooms,
		Activities:   activities,
		RoomSessions: movebinding.NewRoomSessions(presence),
		Moves:        movebinding.NewMoves(presence, checkIns),
		Observe: func(o openRoomMoveCompose.Observation) {
			log.Debug("open room move operation",
				"operation", o.Operation,
				"duration", o.Duration,
				"moved", o.Moved,
				"error", o.Err,
			)
		},
	})
}
