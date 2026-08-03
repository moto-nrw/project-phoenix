package schedule

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
)

func validateAssignablePlanningTrack(
	ctx context.Context,
	repo model.PlanningTrackRepository,
	id *int64,
	allowedArchivedID *int64,
) error {
	if id == nil {
		return nil
	}
	if *id <= 0 || repo == nil {
		return ErrPlanningTrackNotFound
	}
	track, err := repo.FindByIDForShare(ctx, *id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return ErrPlanningTrackNotFound
		}
		return err
	}
	if track.IsArchived() && (allowedArchivedID == nil || *allowedArchivedID != track.ID) {
		return ErrPlanningTrackArchived
	}
	return nil
}

func samePlanningTrackID(left, right *int64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
