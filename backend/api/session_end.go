package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	sessionEndCompose "github.com/moto-nrw/project-phoenix/workflows/sessionend/compose"
)

// newSessionEnd composes the session end workflow (#2697) over the owner
// facades the root already holds: Student Presence, Timetable & Activities,
// People Directory, Facilities, the Timetable owner's bridged completion, the
// realtime hub, and the guardian wake of parent messaging.
func newSessionEnd(presence *studentpresence.Module, modules moduleServices, factory *services.Factory, logger *slog.Logger) (sessionend.Command, error) {
	log := logger.With("workflow", "session-end")
	deps := sessionEndCompose.Dependencies{
		Presence:    presence,
		Timetable:   modules.timetable,
		Completion:  factory.TimetableBridge,
		Students:    modules.persons,
		Rooms:       modules.rooms,
		Broadcaster: factory.RealtimeHub,
		Logger:      log,
		Observe: func(o sessionEndCompose.Observation) {
			observability.ObserveSessionEndOperation(o.Operation, o.Duration, o.Err)
			log.Debug("session end operation",
				"operation", o.Operation,
				"duration", o.Duration,
				"students_checked_out", o.StudentsCheckedOut,
				"supervisors_ended", o.SupervisorsEnded,
				"instance_completed", o.InstanceCompleted,
				"error", o.Err,
			)
		},
	}
	// A typed nil pointer would satisfy the interface and panic on use.
	if factory.ParentEventEmitter != nil {
		deps.Guardians = factory.ParentEventEmitter
	}
	return sessionEndCompose.New(deps)
}
