package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	boundaryBinding "github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewPlanningTrackAdministration(tracks timetable.PlanningTrackCapability, db *bun.DB) timetable.PlanningTrackAdministration {
	return application.NewPlanningTrackAdministration(tracks, (&planningTrackAdministration{}).OrderPlanningTracks(db), tenant.FromContext, db != nil)
}

type planningTrackAdministration struct{}

// OrderPlanningTracks binds the existing registered tenant executor. The
// application owns validation and error translation; this callback only
// binds the database and adapts the callback signature.
func (*planningTrackAdministration) OrderPlanningTracks(db *bun.DB) func(context.Context, int64, func(context.Context) error) error {
	return boundaryBinding.PlanningTrackAdministrationOrderPlanningTracksBinding(db, tenant.WithTenantTx)
}
