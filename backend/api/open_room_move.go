package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	openRoomMoveCompose "github.com/moto-nrw/project-phoenix/workflows/openroommove/compose"
)

// newOpenRoomMove composes the open-room move workflow (#3066) over the owner
// facades the root already holds: Facilities for the release, Timetable &
// Activities for the room's system activity, and the retained Student
// Presence service for the room session and the move until that owner exposes
// them publicly.
func newOpenRoomMove(modules moduleServices, presence openRoomMoveCompose.RetainedPresence, logger *slog.Logger) (openroommove.Command, error) {
	log := logger.With("workflow", "open-room-move")
	return openRoomMoveCompose.New(openRoomMoveCompose.Dependencies{
		Rooms:      modules.rooms,
		Activities: modules.timetable,
		Presence:   presence,
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
