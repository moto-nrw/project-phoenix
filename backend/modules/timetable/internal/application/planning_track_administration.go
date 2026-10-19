package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type PlanningTrackAdministration struct {
	*PlanningTrackViews
	tracks     timetable.PlanningTrackCapability
	execute    func(context.Context, int64, func(context.Context) error) error
	tenantID   func(context.Context) int64
	configured bool
}

func NewPlanningTrackAdministration(tracks timetable.PlanningTrackCapability, execute func(context.Context, int64, func(context.Context) error) error, tenantID func(context.Context) int64, configured bool) *PlanningTrackAdministration {
	return &PlanningTrackAdministration{PlanningTrackViews: NewPlanningTrackViews(tracks), tracks: tracks, execute: execute, tenantID: tenantID, configured: configured}
}

func (a *PlanningTrackAdministration) OrderPlanningTracks(ctx context.Context, ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists || id <= 0 {
			return timetable.ErrInvalidPlanningTrack
		}
		seen[id] = struct{}{}
	}
	tenantID := a.tenantID(ctx)
	if tenantID <= 0 || !a.configured {
		return errors.New("planning track service is not configured")
	}
	return a.execute(ctx, tenantID, func(txCtx context.Context) error {
		err := a.tracks.ReorderPlanningTracks(txCtx, ids)
		if errors.Is(err, timetable.ErrPlanningTrackNotFound) {
			return timetable.ErrPlanningTrackNotFound
		}
		return err
	})
}
