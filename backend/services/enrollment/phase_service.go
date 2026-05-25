package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// PhaseService sentinel errors. The HTTP layer maps these to status
// codes; tests assert on them via errors.Is.
var (
	ErrPhaseNotFound      = errors.New("phase not found")
	ErrInvalidPhase       = errors.New("invalid phase")
	ErrPhaseHasReferences = errors.New("phase still referenced by submissions or care offerings")
	// ErrPhaseHasRequests is returned when the phase still has one or
	// more enrollment.requests rows. Wraps ErrPhaseHasReferences so
	// callers using errors.Is(_, ErrPhaseHasReferences) still match.
	ErrPhaseHasRequests = fmt.Errorf("%w: phase has enrollment requests", ErrPhaseHasReferences)
	// ErrPhaseHasOfferings is returned when the phase still has one or
	// more enrollment.care_offerings rows. Wraps ErrPhaseHasReferences
	// for backward compat.
	ErrPhaseHasOfferings = fmt.Errorf("%w: phase has care offerings", ErrPhaseHasReferences)
)

// PhaseService manages the per-tenant catalog of enrollment phases.
// Each phase carries its own enrollment window, optional form schema,
// per-phase care-overflow mode, and admin/parent visibility flags.
//
// The service deliberately doesn't bake "current school year" logic in
// - admins can have multiple active phases simultaneously (e.g.
// "Schuljahr 26/27" + "Sommerferien 2026"), and the parent landing page
// surfaces every is_active=true phase whose window includes now.
type PhaseService interface {
	List(ctx context.Context) ([]*enrollmentModels.Phase, error)
	ListPublicOpen(ctx context.Context, now time.Time) ([]*enrollmentModels.Phase, error)
	GetByID(ctx context.Context, id int64) (*enrollmentModels.Phase, error)

	Create(ctx context.Context, phase *enrollmentModels.Phase) (*enrollmentModels.Phase, error)
	Update(ctx context.Context, phase *enrollmentModels.Phase) error
	Delete(ctx context.Context, id int64) error
}

// PhaseServiceConfig is the dep-injection bundle.
type PhaseServiceConfig struct {
	Repo             enrollmentModels.PhaseRepository
	RequestRepo      enrollmentModels.RequestRepository
	CareOfferingRepo enrollmentModels.CareOfferingRepository
	Logger           *slog.Logger
}

type phaseService struct {
	repo             enrollmentModels.PhaseRepository
	requestRepo      enrollmentModels.RequestRepository
	careOfferingRepo enrollmentModels.CareOfferingRepository
	logger           *slog.Logger
}

func NewPhaseService(cfg PhaseServiceConfig) PhaseService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &phaseService{
		repo:             cfg.Repo,
		requestRepo:      cfg.RequestRepo,
		careOfferingRepo: cfg.CareOfferingRepo,
		logger:           logger,
	}
}

func (s *phaseService) List(ctx context.Context) ([]*enrollmentModels.Phase, error) {
	return s.repo.ListByTenant(ctx)
}

func (s *phaseService) ListPublicOpen(ctx context.Context, now time.Time) ([]*enrollmentModels.Phase, error) {
	if now.IsZero() {
		now = time.Now()
	}
	return s.repo.ListPublicOpen(ctx, now)
}

func (s *phaseService) GetByID(ctx context.Context, id int64) (*enrollmentModels.Phase, error) {
	if id <= 0 {
		return nil, ErrPhaseNotFound
	}
	phase, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrPhaseNotFound
	}
	return phase, nil
}

func (s *phaseService) Create(ctx context.Context, phase *enrollmentModels.Phase) (*enrollmentModels.Phase, error) {
	if phase == nil {
		return nil, fmt.Errorf("%w: phase is required", ErrInvalidPhase)
	}
	if err := s.repo.Create(ctx, phase); err != nil {
		return nil, err
	}
	s.logger.Info("phase created",
		slog.Int64("phase_id", phase.ID),
		slog.String("name", phase.Name),
		slog.String("kind", phase.Kind))
	return phase, nil
}

func (s *phaseService) Update(ctx context.Context, phase *enrollmentModels.Phase) error {
	if phase == nil || phase.ID <= 0 {
		return fmt.Errorf("%w: phase with valid id is required", ErrInvalidPhase)
	}
	if err := s.repo.Update(ctx, phase); err != nil {
		return err
	}
	s.logger.Info("phase updated", slog.Int64("phase_id", phase.ID))
	return nil
}

// Delete refuses to remove phases that already have rows referencing
// them. Admins should toggle is_active=false instead - that hides the
// phase from parents without losing the audit trail.
//
// The reference checks short-circuit: requests first (more likely to
// be set), then care offerings.
func (s *phaseService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidPhase)
	}

	// Both checks bypass the FK ON DELETE CASCADE - we want a clean
	// admin-facing error, not a silent cascade that nukes submissions.
	hasRequests, err := s.phaseHasRequests(ctx, id)
	if err != nil {
		return fmt.Errorf("phase delete: check requests: %w", err)
	}
	if hasRequests {
		return ErrPhaseHasRequests
	}
	hasOfferings, err := s.phaseHasOfferings(ctx, id)
	if err != nil {
		return fmt.Errorf("phase delete: check care offerings: %w", err)
	}
	if hasOfferings {
		return ErrPhaseHasOfferings
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("phase deleted", slog.Int64("phase_id", id))
	return nil
}

func (s *phaseService) phaseHasRequests(ctx context.Context, phaseID int64) (bool, error) {
	if s.requestRepo == nil {
		return false, nil
	}
	return s.requestRepo.ExistsByPhaseID(ctx, phaseID)
}

func (s *phaseService) phaseHasOfferings(ctx context.Context, phaseID int64) (bool, error) {
	if s.careOfferingRepo == nil {
		return false, nil
	}
	list, err := s.careOfferingRepo.ListByPhase(ctx, phaseID)
	if err != nil {
		return false, err
	}
	return len(list) > 0, nil
}
