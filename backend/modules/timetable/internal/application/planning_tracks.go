package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindPlanningTrack(ctx context.Context, id int64, lock string) (result domain.PlanningTrack, err error) {
	err = s.run("find_planning_track", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindPlanningTrack(ctx, id, lock)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrPlanningTrackNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListPlanningTracks(ctx context.Context, filter domain.PlanningTrackFilter) (result []domain.PlanningTrack, err error) {
	err = s.run("list_planning_tracks", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlanningTracks(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreatePlanningTrack(ctx context.Context, fields domain.PlanningTrackFields) (result domain.PlanningTrack, err error) {
	err = s.runWrite(ctx, "create_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreatePlanningTrack(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdatePlanningTrack(ctx context.Context, id int64, fields domain.PlanningTrackFields, activeOnly bool) (result domain.PlanningTrack, updated bool, err error) {
	err = s.runWrite(ctx, "update_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdatePlanningTrack(txCtx, id, fields, activeOnly)
		stats.Add(queryStats)
		result, updated = value, found
		if updateErr != nil || found || activeOnly {
			return updateErr
		}
		return domain.ErrPlanningTrackNotFound
	})
	return result, updated, err
}

func (s *Service) DeletePlanningTrack(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePlanningTrack(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (result domain.PlanningTrack, updated bool, err error) {
	err = s.runWrite(ctx, "archive_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		track, found, queryStats, updateErr := s.store.SetPlanningTrackArchivedAt(txCtx, id, value)
		stats.Add(queryStats)
		result, updated = track, found
		return updateErr
	})
	return result, updated, err
}

func (s *Service) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	return s.runWrite(ctx, "reorder_planning_tracks", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.ReorderPlanningTracks(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (result domain.PlanningTrack, restored bool, err error) {
	err = s.runWrite(ctx, "restore_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		track, found, queryStats, restoreErr := s.store.RestorePlanningTrackAtEnd(txCtx, id)
		stats.Add(queryStats)
		result, restored = track, found
		return restoreErr
	})
	return result, restored, err
}
