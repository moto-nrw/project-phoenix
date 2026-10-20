package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindPlanningTrack(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	return s.findPlanningTrack(ctx, id, "")
}

func (s *Service) FindPlanningTrackForShare(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	return s.findPlanningTrack(ctx, id, "SHARE")
}

func (s *Service) findPlanningTrack(ctx context.Context, id int64, lock string) (result timetable.PlanningTrack, err error) {
	if id <= 0 {
		return timetable.PlanningTrack{}, s.reject("find_planning_track", timetable.ErrInvalidPlanningTrackQuery)
	}
	err = s.run(ctx, "find_planning_track", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindPlanningTrack(ctx, id, lock)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrPlanningTrackNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListPlanningTracks(ctx context.Context, filter timetable.PlanningTrackFilter) (result []timetable.PlanningTrack, err error) {
	if hasInvalidID(filter.IDs) {
		return nil, s.reject("list_planning_tracks", timetable.ErrInvalidPlanningTrackQuery)
	}

	err = s.run(ctx, "list_planning_tracks", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListPlanningTracks(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreatePlanningTrack(ctx context.Context, fields timetable.PlanningTrackInput) (result timetable.PlanningTrack, err error) {
	fields, ok := normalizePlanningTrack(fields)
	if !ok {
		return timetable.PlanningTrack{}, s.reject("create_planning_track", timetable.ErrInvalidPlanningTrack)
	}

	err = s.runWrite(ctx, "create_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreatePlanningTrack(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdatePlanningTrack(ctx context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	track, _, err := s.updatePlanningTrack(ctx, id, input, false)
	return track, err
}

func (s *Service) UpdateActivePlanningTrack(ctx context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, bool, error) {
	return s.updatePlanningTrack(ctx, id, input, true)
}

func (s *Service) updatePlanningTrack(ctx context.Context, id int64, fields timetable.PlanningTrackInput, activeOnly bool) (result timetable.PlanningTrack, updated bool, err error) {
	fields, ok := normalizePlanningTrack(fields)
	if id <= 0 || !ok {
		return timetable.PlanningTrack{}, false, s.reject("update_planning_track", timetable.ErrInvalidPlanningTrack)
	}
	err = s.runWrite(ctx, "update_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdatePlanningTrack(txCtx, id, fields, activeOnly)
		stats.Add(queryStats)
		result, updated = value, found
		if updateErr != nil || found || activeOnly {
			return updateErr
		}
		return timetable.ErrPlanningTrackNotFound
	})
	return result, updated, err
}

func (s *Service) DeletePlanningTrack(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_planning_track", timetable.ErrInvalidPlanningTrack)
	}

	return s.runWrite(ctx, "delete_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeletePlanningTrack(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SetPlanningTrackArchivedAt(ctx context.Context, id int64, value *time.Time) (result timetable.PlanningTrack, updated bool, err error) {
	if id <= 0 {
		return timetable.PlanningTrack{}, false, s.reject("archive_planning_track", timetable.ErrInvalidPlanningTrack)
	}

	err = s.runWrite(ctx, "archive_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		track, found, queryStats, updateErr := s.store.SetPlanningTrackArchivedAt(txCtx, id, value)
		stats.Add(queryStats)
		result, updated = track, found
		return updateErr
	})
	return result, updated, err
}

func (s *Service) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	if hasInvalidID(ids) || hasDuplicateID(ids) {
		return s.reject("reorder_planning_tracks", timetable.ErrInvalidPlanningTrack)
	}

	return s.runWrite(ctx, "reorder_planning_tracks", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.ReorderPlanningTracks(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) RestorePlanningTrackAtEnd(ctx context.Context, id int64) (result timetable.PlanningTrack, restored bool, err error) {
	if id <= 0 {
		return timetable.PlanningTrack{}, false, s.reject("restore_planning_track", timetable.ErrInvalidPlanningTrack)
	}

	err = s.runWrite(ctx, "restore_planning_track", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		track, found, queryStats, restoreErr := s.store.RestorePlanningTrackAtEnd(txCtx, id)
		stats.Add(queryStats)
		result, restored = track, found
		return restoreErr
	})
	return result, restored, err
}
