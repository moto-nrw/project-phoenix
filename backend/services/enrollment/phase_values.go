package enrollment

import (
	"context"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type RolloverPhases interface {
	Phase(context.Context, int64) (*capability.Phase, error)
	InsertPhase(context.Context, *capability.Phase) error
	HasRolloverSuccessor(context.Context, int64) (bool, error)
	PhasesWithExpiredRolloverDeadline(context.Context, time.Time) ([]*capability.Phase, error)
}

type PhaseReader interface {
	Phase(context.Context, int64) (*capability.Phase, error)
	Phases(context.Context) ([]*capability.Phase, error)
}

type PhaseBatchReader interface {
	PhaseReader
	PhasesByID(context.Context, []int64) ([]*capability.Phase, error)
}
