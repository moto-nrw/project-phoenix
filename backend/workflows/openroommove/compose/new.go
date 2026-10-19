// Package compose binds the open-room move workflow to the tenant runtime and
// the owner capabilities.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/ports"
)

type Observation = ports.Observation

// Dependencies are the owner capabilities the workflow coordinates.
type Dependencies struct {
	Rooms        ports.Rooms
	Activities   ports.Activities
	RoomSessions ports.RoomSessions
	Moves        ports.Moves
	Observe      func(Observation)
}

// New binds the workflow. It joins the caller's tenant transaction when one
// is open and otherwise runs the move in its own.
func New(deps Dependencies) (openroommove.Command, error) {
	if deps.Rooms == nil || deps.Activities == nil || deps.RoomSessions == nil || deps.Moves == nil || deps.Observe == nil {
		return nil, errors.New("open room move compose: all dependencies are required")
	}
	return application.NewCommand(application.Dependencies{
		Rooms:        deps.Rooms,
		Activities:   deps.Activities,
		RoomSessions: deps.RoomSessions,
		Moves:        deps.Moves,
		Runtime: ports.Runtime{
			TenantID:     tenant.FromContext,
			WithinTenant: tenant.WithinCurrentTenant,
			AcquireLock: func(ctx context.Context, key string) error {
				return tenant.AcquireLock(ctx, key, false)
			},
		},
		Observe: deps.Observe,
	}), nil
}
