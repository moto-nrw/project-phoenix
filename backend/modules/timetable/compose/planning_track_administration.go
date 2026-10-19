package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// planningTrackAdministration serves timetable.PlanningTrackAdministration
// over the owner's planning-track capability (#2383, #3424 slice S5).
type planningTrackAdministration struct {
	*application.PlanningTrackViews
	tracks timetable.PlanningTrackCapability
	db     *bun.DB
}

// NewPlanningTrackAdministration composes the planning-track editor. db runs
// the reorder in one tenant transaction; without it reordering fails with a
// configuration error.
func NewPlanningTrackAdministration(tracks timetable.PlanningTrackCapability, db *bun.DB) timetable.PlanningTrackAdministration {
	return &planningTrackAdministration{PlanningTrackViews: application.NewPlanningTrackViews(tracks), tracks: tracks, db: db}
}

func (a *planningTrackAdministration) OrderPlanningTracks(ctx context.Context, ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists || id <= 0 {
			return timetable.ErrInvalidPlanningTrack
		}
		seen[id] = struct{}{}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 || a.db == nil {
		return errors.New("planning track service is not configured")
	}
	return tenant.WithTenantTx(ctx, a.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		err := a.tracks.ReorderPlanningTracks(txCtx, ids)
		if errors.Is(err, timetable.ErrPlanningTrackNotFound) {
			return timetable.ErrPlanningTrackNotFound
		}
		return err
	})
}
