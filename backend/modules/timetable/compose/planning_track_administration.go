package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type planningTrackAdministration struct{}

func NewPlanningTrackAdministration(tracks timetable.PlanningTrackCapability, db *bun.DB) timetable.PlanningTrackAdministration {
	return application.NewPlanningTrackAdministration(tracks, (&planningTrackAdministration{}).OrderPlanningTracks(db), tenant.FromContext, db != nil)
}

// OrderPlanningTracks binds the existing registered tenant executor. The
// application owns validation and error translation; this callback only
// binds the database and adapts the callback signature.
func (*planningTrackAdministration) OrderPlanningTracks(db *bun.DB) func(context.Context, int64, func(context.Context) error) error {
	return func(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
		return tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return callback(txCtx) })
	}
}
