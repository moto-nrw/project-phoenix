// Package compose binds the session end workflow to the tenant runtime, the
// owner capabilities, and the realtime delivery platform.
package compose

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend/ports"
)

type Observation = ports.Observation

// Dependencies are the owner capabilities the workflow coordinates. The
// modules' public facades satisfy the port interfaces directly.
type Dependencies struct {
	Presence   ports.Presence
	Sessions   ports.Sessions
	Timetable  ports.Timetable
	Completion ports.InstanceCompletion
	Students   ports.Students
	Rooms      ports.Rooms
	Notifier   ports.Notifier
	Observe    func(Observation)
	// Now is optional and defaults to the wall clock.
	Now func() time.Time
}

// New binds the workflow. It joins the caller's tenant transaction when one
// is open and otherwise runs the close in its own.
func New(deps Dependencies) (sessionend.Command, error) {
	if deps.Presence == nil || deps.Sessions == nil || deps.Timetable == nil || deps.Completion == nil || deps.Students == nil ||
		deps.Rooms == nil || deps.Notifier == nil || deps.Observe == nil {
		return nil, errors.New("session end compose: all dependencies are required")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return application.NewCommand(application.Dependencies{
		Presence:   deps.Presence,
		Sessions:   deps.Sessions,
		Timetable:  deps.Timetable,
		Completion: deps.Completion,
		Students:   deps.Students,
		Rooms:      deps.Rooms,
		Notifier:   deps.Notifier,
		Runtime: ports.Runtime{
			TenantID:     tenant.FromContext,
			WithinTenant: tenant.WithinCurrentTenant,
			AfterCommit:  tenant.RegisterAfterCommit,
			Now:          now,
		},
		Observe: deps.Observe,
	}), nil
}
