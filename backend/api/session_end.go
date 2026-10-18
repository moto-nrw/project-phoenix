package api

import (
	"log/slog"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	sessionEndCompose "github.com/moto-nrw/project-phoenix/workflows/sessionend/compose"
)

// newSessionEnd composes the session end workflow (#2697) over the owner
// facades the root already holds: Student Presence, Timetable & Activities,
// People Directory, Facilities, the Timetable owner's bridged completion, the
// realtime hub, and the guardian wake of parent messaging.
func newSessionEnd(
	presence *studentpresence.Module, activities timetable.Capability, completion timetable.EndedSessionCompletion,
	students peopledirectory.Capability, rooms *facilitiesModule.Module, hub *realtime.Hub,
	guardians eventContract.Emitter, logger *slog.Logger,
) (sessionend.Command, error) {
	log := logger.With("workflow", "session-end")
	deps := sessionEndCompose.Dependencies{
		Presence:    presence,
		Sessions:    presence,
		Timetable:   activities,
		Completion:  completion,
		Students:    students,
		Rooms:       rooms,
		Broadcaster: hub,
		Guardians:   guardians,
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
	return sessionEndCompose.New(deps)
}
