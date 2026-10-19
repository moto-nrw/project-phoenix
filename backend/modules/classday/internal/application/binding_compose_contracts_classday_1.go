package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupBaselineReader is the retained date-aware recurring pickup projection
// (no exceptions applied).
type PickupBaselineReader interface {
	Project(ctx context.Context, studentIDs []int64, from, to calendar.Date) (*careplan.PickupBaselineProjection, error)
}
