package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
)

// WorkTimeModelUsage counts the live assignees per template in a statement
// count independent of the number of templates and staff.
func (s *Service) WorkTimeModelUsage(ctx context.Context) (map[int64]int, error) {
	usage := map[int64]int{}
	err := s.run(ctx, "work_time_model_usage", func(ctx context.Context, _ *domain.OperationStats) error {
		byModel, err := s.assignments.AssignedStaffByModel(ctx)
		if err != nil {
			return err
		}
		for modelID, staffIDs := range byModel {
			if len(staffIDs) > 0 {
				usage[modelID] = len(staffIDs)
			}
		}
		return nil
	})
	if err != nil {
		return nil, MapError(err)
	}
	return usage, nil
}

func (s *Service) ArchiveWorkTimeModel(ctx context.Context, id int64) (workforce.WorkTimeModel, error) {
	return s.setWorkTimeModelArchived(ctx, "archive_work_time_model", id, true)
}

func (s *Service) RestoreWorkTimeModel(ctx context.Context, id int64) (workforce.WorkTimeModel, error) {
	return s.setWorkTimeModelArchived(ctx, "restore_work_time_model", id, false)
}

// setWorkTimeModelArchived changes only the offer state. The assignees and
// their schedule versions stay as they are, so no Soll changes.
func (s *Service) setWorkTimeModelArchived(ctx context.Context, operation string, id int64, archived bool) (workforce.WorkTimeModel, error) {
	if id <= 0 {
		return workforce.WorkTimeModel{}, invalid("work time model ID is required")
	}
	var result domain.WorkTimeModel
	// One UPDATE plus its reload: the unit of work s.run already opens (or
	// joins) keeps both together, so no second boundary is needed.
	err := s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		model, found, writeStats, err := s.store.SetWorkTimeModelArchived(ctx, id, archived)
		stats.Add(writeStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrWorkTimeModelNotFound
		}
		result = model
		return nil
	})
	return modelToPublic(result), MapError(err)
}
