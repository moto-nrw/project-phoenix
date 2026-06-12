package config

import (
	"context"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// WorkTimeModelService exposes work-time-model CRUD to the api layer (issue
// #584: handlers must not hold repositories). Lookup results and errors are
// returned verbatim; the handlers keep their existing error-to-status
// mapping (e.g. delete's blanket "not found or in use" 404).
type WorkTimeModelService interface {
	// ListModels retrieves all work time models with entries.
	ListModels(ctx context.Context) ([]*configModels.WorkTimeModel, error)

	// GetModel retrieves a work time model by ID.
	GetModel(ctx context.Context, id int64) (*configModels.WorkTimeModel, error)

	// CreateModel persists a new model with its entries and returns the
	// freshly reloaded row.
	CreateModel(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) (*configModels.WorkTimeModel, error)

	// UpdateModel replaces the model and its entries, refreshes the schedule
	// snapshots of all staff bound to the template, and returns the reloaded
	// row.
	UpdateModel(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) (*configModels.WorkTimeModel, error)

	// DeleteModel removes a model. The repository error is returned verbatim
	// (the handler maps every failure to its historical 404 message).
	DeleteModel(ctx context.Context, id int64) error
}

type workTimeModelService struct {
	repo configModels.WorkTimeModelRepository
}

// NewWorkTimeModelService creates a WorkTimeModelService backed by the
// work-time-model repository.
func NewWorkTimeModelService(repo configModels.WorkTimeModelRepository) WorkTimeModelService {
	return &workTimeModelService{repo: repo}
}

func (s *workTimeModelService) ListModels(ctx context.Context) ([]*configModels.WorkTimeModel, error) {
	return s.repo.List(ctx)
}

func (s *workTimeModelService) GetModel(ctx context.Context, id int64) (*configModels.WorkTimeModel, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *workTimeModelService) CreateModel(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) (*configModels.WorkTimeModel, error) {
	if err := s.repo.Create(ctx, model, entries); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, model.ID)
}

func (s *workTimeModelService) UpdateModel(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) (*configModels.WorkTimeModel, error) {
	if err := s.repo.Update(ctx, model, entries); err != nil {
		return nil, err
	}
	if err := s.repo.RefreshAssignedStaffSchedules(ctx, model.ID); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, model.ID)
}

func (s *workTimeModelService) DeleteModel(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
