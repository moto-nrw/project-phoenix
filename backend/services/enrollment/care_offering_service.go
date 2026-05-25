package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// ErrCareOfferingNotFound is the sentinel returned by GetByID when the
// row doesn't exist (or the tenant can't see it via RLS).
var ErrCareOfferingNotFound = errors.New("care offering not found")

// CareOfferingService manages the per-tenant care-offering catalog.
// Admin endpoints (PR 6) read + write all offerings; the public form
// fetches the active offerings for a parent-selected phase.
type CareOfferingService interface {
	List(ctx context.Context) ([]*enrollmentModels.CareOffering, error)
	ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error)
	GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error)
	Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error)
	Update(ctx context.Context, offering *enrollmentModels.CareOffering) error
	Delete(ctx context.Context, id int64) error

	// Clone copies an existing offering into a new row scoped to a
	// target phase. All offering-level fields (capacity, days, lunch,
	// price, etc.) are preserved; the source row's ID is reset and
	// phase_id is set to the target. Use case: cloning last year's
	// catalog into this year's phase, then editing what changed.
	Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*enrollmentModels.CareOffering, error)
}

// CareOfferingServiceConfig is the dep-injection bundle.
type CareOfferingServiceConfig struct {
	Repo   enrollmentModels.CareOfferingRepository
	Logger *slog.Logger
}

type careOfferingService struct {
	repo   enrollmentModels.CareOfferingRepository
	logger *slog.Logger
}

// NewCareOfferingService builds the service.
func NewCareOfferingService(cfg CareOfferingServiceConfig) CareOfferingService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &careOfferingService{
		repo:   cfg.Repo,
		logger: logger,
	}
}

func (s *careOfferingService) List(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	return s.repo.ListByTenant(ctx)
}

func (s *careOfferingService) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase_id must be positive")
	}
	return s.repo.ListByPhase(ctx, phaseID)
}

func (s *careOfferingService) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase_id must be positive")
	}
	return s.repo.ListActiveByPhase(ctx, phaseID)
}

func (s *careOfferingService) GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	if id <= 0 {
		return nil, ErrCareOfferingNotFound
	}
	offering, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrCareOfferingNotFound
	}
	return offering, nil
}

func (s *careOfferingService) Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	if offering == nil {
		return nil, fmt.Errorf("offering is required")
	}
	if err := s.repo.Create(ctx, offering); err != nil {
		return nil, err
	}
	s.logger.Info("care offering created",
		slog.Int64("offering_id", offering.ID),
		slog.String("name", offering.Name))
	return offering, nil
}

func (s *careOfferingService) Update(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil || offering.ID <= 0 {
		return fmt.Errorf("offering with valid id is required")
	}
	if err := s.repo.Update(ctx, offering); err != nil {
		return err
	}
	s.logger.Info("care offering updated", slog.Int64("offering_id", offering.ID))
	return nil
}

func (s *careOfferingService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("id must be positive")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("care offering deleted", slog.Int64("offering_id", id))
	return nil
}

// Clone copies a care offering into a new row scoped to a target phase.
// All offering-level fields are preserved; ID is reset so the DB
// assigns a fresh BIGSERIAL, and phase_id is repointed at the target.
func (s *careOfferingService) Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*enrollmentModels.CareOffering, error) {
	if sourceID <= 0 {
		return nil, fmt.Errorf("source id must be positive")
	}
	if targetPhaseID <= 0 {
		return nil, fmt.Errorf("target phase id must be positive")
	}

	source, err := s.repo.FindByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("clone: source lookup: %w", err)
	}

	clone := *source
	clone.ID = 0 // BIGSERIAL - let the DB assign
	clone.PhaseID = targetPhaseID

	if err := s.repo.Create(ctx, &clone); err != nil {
		return nil, fmt.Errorf("clone: create: %w", err)
	}
	s.logger.Info("care offering cloned",
		slog.Int64("source_id", sourceID),
		slog.Int64("clone_id", clone.ID),
		slog.Int64("target_phase_id", targetPhaseID))
	return &clone, nil
}
