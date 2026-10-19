package api

import (
	"log/slog"

	movebinding "github.com/moto-nrw/project-phoenix/roots/openroommovebinding"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"

	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	openRoomMoveCompose "github.com/moto-nrw/project-phoenix/workflows/openroommove/compose"
)

// newOpenRoomMove composes the open-room move workflow (#3066) over the owner
// facades the root already holds: Facilities for the release, Timetable &
// Activities for the room's system activity, and the retained Student
// Presence service for the room session and the move until that owner exposes
// them publicly.
func newOpenRoomMove(rooms *facilitiesModule.Module, activities timetable.Capability, presence movebinding.Presence, logger *slog.Logger) (openroommove.Command, error) {
	log := logger.With("workflow", "open-room-move")
	return openRoomMoveCompose.New(openRoomMoveCompose.Dependencies{
		Rooms:        rooms,
		Activities:   activities,
		RoomSessions: movebinding.NewRoomSessions(presence),
		Moves:        movebinding.NewMoves(presence),
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
