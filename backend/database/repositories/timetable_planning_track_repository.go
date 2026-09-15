package repositories

import (
	"context"
	"errors"
	"fmt"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type timetablePlanningTrackRepository struct {
	timetable timetable.PlanningTrackCapability
}

func (r timetablePlanningTrackRepository) Create(ctx context.Context, value *scheduleModels.PlanningTrack) error {
	if value == nil {
		return errors.New("planning track cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreatePlanningTrack(ctx, publicPlanningTrackInput(value))
	if err != nil {
		return scheduleRepo.WrapDatabaseError("create", err)
	}
	replaceLegacyPlanningTrack(value, created)
	return nil
}

func (r timetablePlanningTrackRepository) FindByID(ctx context.Context, id any) (*scheduleModels.PlanningTrack, error) {
	trackID, ok := legacyGroupID(id)
	if !ok {
		return nil, scheduleRepo.WrapDatabaseError("find by id", fmt.Errorf("invalid planning track id %T", id))
	}
	value, err := r.timetable.FindPlanningTrack(ctx, trackID)
	if err != nil {
		return nil, legacyPlanningTrackError("find by id", err)
	}
	return legacyPlanningTrack(value), nil
}

func (r timetablePlanningTrackRepository) Update(ctx context.Context, value *scheduleModels.PlanningTrack) error {
	if value == nil {
		return errors.New("planning track cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdatePlanningTrack(ctx, value.ID, publicPlanningTrackInput(value))
	if err != nil {
		return legacyPlanningTrackError("update", err)
	}
	replaceLegacyPlanningTrack(value, updated)
	return nil
}

func (r timetablePlanningTrackRepository) Delete(ctx context.Context, id any) error {
	trackID, ok := legacyGroupID(id)
	if !ok {
		return scheduleRepo.WrapDatabaseError("delete", fmt.Errorf("invalid planning track id %T", id))
	}
	if err := r.timetable.DeletePlanningTrack(ctx, trackID); err != nil {
		return scheduleRepo.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetablePlanningTrackRepository) List(ctx context.Context, filters map[string]any) ([]*scheduleModels.PlanningTrack, error) {
	for _, value := range filters {
		if value != nil {
			return nil, scheduleRepo.WrapDatabaseError("list", errors.New("planning track filters are unsupported"))
		}
	}
	return r.list(ctx, timetable.PlanningTrackFilter{}, "list")
}

func (r timetablePlanningTrackRepository) ListAll(ctx context.Context) ([]*scheduleModels.PlanningTrack, error) {
	return r.list(ctx, timetable.PlanningTrackFilter{Ordered: true}, "list planning tracks")
}

func (r timetablePlanningTrackRepository) FindByIDs(ctx context.Context, ids []int64) ([]*scheduleModels.PlanningTrack, error) {
	if len(ids) == 0 {
		return []*scheduleModels.PlanningTrack{}, nil
	}
	return r.list(ctx, timetable.PlanningTrackFilter{IDs: ids}, "find planning tracks by ids")
}

func (r timetablePlanningTrackRepository) FindByIDForShare(ctx context.Context, id int64) (*scheduleModels.PlanningTrack, error) {
	value, err := r.timetable.FindPlanningTrackForShare(ctx, id)
	if err != nil {
		return nil, legacyPlanningTrackError("find planning track for share", err)
	}
	return legacyPlanningTrack(value), nil
}

func (r timetablePlanningTrackRepository) UpdateIfActive(ctx context.Context, value *scheduleModels.PlanningTrack) (bool, error) {
	if value == nil {
		return false, errors.New("planning track cannot be nil")
	}
	if err := value.Validate(); err != nil {
		return false, err
	}
	updated, ok, err := r.timetable.UpdateActivePlanningTrack(ctx, value.ID, publicPlanningTrackInput(value))
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("update active planning track", err)
	}
	if ok {
		replaceLegacyPlanningTrack(value, updated)
	}
	return ok, nil
}

func (r timetablePlanningTrackRepository) UpdateSortOrders(ctx context.Context, ids []int64) error {
	return legacyPlanningTrackError("reorder planning tracks", r.timetable.ReorderPlanningTracks(ctx, ids))
}

func (r timetablePlanningTrackRepository) RestoreAtEnd(ctx context.Context, value *scheduleModels.PlanningTrack) (bool, error) {
	if value == nil {
		return false, errors.New("planning track cannot be nil")
	}
	restored, ok, err := r.timetable.RestorePlanningTrackAtEnd(ctx, value.ID)
	if err != nil {
		return false, scheduleRepo.WrapDatabaseError("restore planning track", err)
	}
	if ok {
		replaceLegacyPlanningTrack(value, restored)
	}
	return ok, nil
}

func (r timetablePlanningTrackRepository) UpdateColumns(ctx context.Context, value *scheduleModels.PlanningTrack, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("planning track cannot be nil or zero value")
	}
	if len(columns) != 1 || columns[0] != "archived_at" {
		return 0, errors.New("planning track compatibility adapter only updates archived_at")
	}
	updated, ok, err := r.timetable.SetPlanningTrackArchivedAt(ctx, value.ID, value.ArchivedAt)
	if err != nil {
		return 0, scheduleRepo.WrapDatabaseError("update columns", err)
	}
	if !ok {
		return 0, nil
	}
	replaceLegacyPlanningTrack(value, updated)
	return 1, nil
}

func (r timetablePlanningTrackRepository) list(ctx context.Context, filter timetable.PlanningTrackFilter, operation string) ([]*scheduleModels.PlanningTrack, error) {
	values, err := r.timetable.ListPlanningTracks(ctx, filter)
	if err != nil {
		return nil, legacyPlanningTrackError(operation, err)
	}
	result := make([]*scheduleModels.PlanningTrack, 0, len(values))
	for _, value := range values {
		result = append(result, legacyPlanningTrack(value))
	}
	return result, nil
}

func legacyPlanningTrack(value timetable.PlanningTrack) *scheduleModels.PlanningTrack {
	result := &scheduleModels.PlanningTrack{Name: value.Name, Color: value.Color, SortOrder: value.SortOrder, ArchivedAt: value.ArchivedAt}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return result
}

func replaceLegacyPlanningTrack(result *scheduleModels.PlanningTrack, value timetable.PlanningTrack) {
	*result = *legacyPlanningTrack(value)
}

func publicPlanningTrackInput(value *scheduleModels.PlanningTrack) timetable.PlanningTrackInput {
	return timetable.PlanningTrackInput{Name: value.Name, Color: value.Color, SortOrder: value.SortOrder, ArchivedAt: value.ArchivedAt}
}

func legacyPlanningTrackError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, timetable.ErrPlanningTrackNotFound) {
		return scheduleRepo.WrapNotFoundDatabaseError(operation)
	}
	return scheduleRepo.WrapDatabaseError(operation, err)
}
