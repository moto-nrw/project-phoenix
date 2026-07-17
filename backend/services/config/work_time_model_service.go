package config

import (
	"context"
	"errors"
	"fmt"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// WorkTimeModelService exposes work-time-model CRUD to the api layer (issue
// #584: handlers must not hold repositories). Lookup results and errors are
// returned verbatim; the handlers keep their existing error-to-status
// mapping (e.g. delete's blanket "not found or in use" 404).
type WorkTimeModelService struct {
	repo configModels.WorkTimeModelRepository
}

// NewWorkTimeModelService creates a WorkTimeModelService backed by the
// work-time-model repository.
func NewWorkTimeModelService(repo configModels.WorkTimeModelRepository) *WorkTimeModelService {
	return &WorkTimeModelService{repo: repo}
}

// ListModels retrieves all work time models with entries.
func (s *WorkTimeModelService) ListModels(ctx context.Context) ([]*configModels.WorkTimeModel, error) {
	return s.repo.List(ctx)
}

// GetModel retrieves a work time model by ID.
func (s *WorkTimeModelService) GetModel(ctx context.Context, id int64) (*configModels.WorkTimeModel, error) {
	return s.repo.FindByID(ctx, id)
}

// CreateModel persists a new model with its entries and returns the
// freshly reloaded row.
func (s *WorkTimeModelService) CreateModel(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) (*configModels.WorkTimeModel, error) {
	if err := validateModelWithEntries(model, entries); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, model, entries); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, model.ID)
}

// validateModelWithEntries enforces the work-time-model invariants for any
// caller (HTTP handlers, CLI, migrations). Entries must pass their own field
// validation, stay within the model's declared rotation length, and never
// double-book a (week_index, day_of_week) slot.
func validateModelWithEntries(model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) error {
	if model == nil {
		return errors.New("model is required")
	}
	if err := model.Validate(); err != nil {
		return err
	}
	seenSlots := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return err
		}
		if entry.WeekIndex >= model.RotationLength {
			return errors.New("entry week_index outside rotation_length")
		}
		slot := fmt.Sprintf("%d:%d", entry.WeekIndex, entry.DayOfWeek)
		if _, exists := seenSlots[slot]; exists {
			return errors.New("duplicate entry for week_index and day_of_week")
		}
		seenSlots[slot] = struct{}{}
	}
	return nil
}

// UpdateModel replaces the model and its entries, refreshes the schedule
// snapshots of all staff bound to the template, and returns the reloaded
// row.
func (s *WorkTimeModelService) UpdateModel(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) (*configModels.WorkTimeModel, error) {
	if err := validateModelWithEntries(model, entries); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, model, entries); err != nil {
		return nil, err
	}
	if err := s.repo.RefreshAssignedStaffSchedules(ctx, model.ID); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, model.ID)
}

// DeleteModel removes a model. The repository error is returned verbatim
// (the handler maps every failure to its historical 404 message).
func (s *WorkTimeModelService) DeleteModel(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
