package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/uptrace/bun"
)

// PhaseService sentinel errors. The HTTP layer maps these to status
// codes; tests assert on them via errors.Is.
var (
	ErrPhaseNotFound = errors.New("phase not found")
	ErrInvalidPhase  = errors.New("invalid phase")
)

// PhaseDeleteImpact summarizes what a phase delete will remove vs keep.
// The admin confirmation modal renders these counts before the
// destructive action: Requests + CareOfferings are permanently deleted,
// StudentsKept survive (their request_children back-link is cleared via
// ON DELETE SET NULL, but the users.students rows are untouched).
type PhaseDeleteImpact struct {
	Requests      int `json:"requests"`
	CareOfferings int `json:"care_offerings"`
	StudentsKept  int `json:"students_kept"`
}

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

	// DeleteImpact reports how many enrollment requests + care offerings a
	// delete would remove, and how many created students would be kept.
	// Backs the admin confirmation modal.
	DeleteImpact(ctx context.Context, id int64) (*PhaseDeleteImpact, error)

	// Delete permanently removes the phase and all of its enrollment
	// records (requests, request children, child-offering selections, and
	// care offerings) in one transaction. Students created from the phase
	// are preserved. There is no reference guard — admins may delete a
	// phase at any point in its lifecycle (before, during, or after the
	// service period); use Update(is_active=false) to merely hide it.
	Delete(ctx context.Context, id int64) error
}

// PhaseServiceConfig is the dep-injection bundle.
type PhaseServiceConfig struct {
	Repo             enrollmentModels.PhaseRepository
	RequestRepo      enrollmentModels.RequestRepository
	RequestChildRepo enrollmentModels.RequestChildRepository
	CareOfferingRepo enrollmentModels.CareOfferingRepository
	// CalendarPeriods validates phase→calendar-period links on
	// Create/Update. Optional: when nil (unit tests with mocks), the
	// link is accepted unvalidated and the FK constraint still holds.
	CalendarPeriods scheduleService.CalendarPeriodService
	DB              *bun.DB
	Logger          *slog.Logger
}

type phaseService struct {
	repo             enrollmentModels.PhaseRepository
	requestRepo      enrollmentModels.RequestRepository
	requestChildRepo enrollmentModels.RequestChildRepository
	careOfferingRepo enrollmentModels.CareOfferingRepository
	calendarPeriods  scheduleService.CalendarPeriodService
	txHandler        *modelBase.TxHandler
	logger           *slog.Logger
}

func NewPhaseService(cfg PhaseServiceConfig) PhaseService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	var txHandler *modelBase.TxHandler
	if cfg.DB != nil {
		txHandler = modelBase.NewTxHandler(cfg.DB)
	}
	return &phaseService{
		repo:             cfg.Repo,
		requestRepo:      cfg.RequestRepo,
		requestChildRepo: cfg.RequestChildRepo,
		careOfferingRepo: cfg.CareOfferingRepo,
		calendarPeriods:  cfg.CalendarPeriods,
		txHandler:        txHandler,
		logger:           logger,
	}
}

// validateCalendarPeriodLink checks that a linked calendar period exists
// for the current tenant. The lookup runs inside the tenant transaction,
// so RLS scopes it — this is what actually blocks cross-tenant links,
// because FK constraint checks bypass RLS.
func (s *phaseService) validateCalendarPeriodLink(ctx context.Context, phase *enrollmentModels.Phase) error {
	if phase.CalendarPeriodID == nil || s.calendarPeriods == nil {
		return nil
	}
	if _, err := s.calendarPeriods.GetPeriodByID(ctx, *phase.CalendarPeriodID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: calendar period %d not found", ErrInvalidPhase, *phase.CalendarPeriodID)
		}
		return fmt.Errorf("validate calendar period link: %w", err)
	}
	return nil
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
	if err := phase.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPhase, err)
	}
	if err := s.validateCalendarPeriodLink(ctx, phase); err != nil {
		return nil, err
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
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPhase, err)
	}
	if err := s.validateCalendarPeriodLink(ctx, phase); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, phase); err != nil {
		return err
	}
	s.logger.Info("phase updated", slog.Int64("phase_id", phase.ID))
	return nil
}

// DeleteImpact returns the blast radius of deleting the phase so the
// admin UI can warn before the destructive action. Requests +
// CareOfferings are deleted; StudentsKept survive.
func (s *phaseService) DeleteImpact(ctx context.Context, id int64) (*PhaseDeleteImpact, error) {
	if id <= 0 {
		return nil, ErrPhaseNotFound
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, ErrPhaseNotFound
	}

	impact := &PhaseDeleteImpact{}
	var err error
	if s.requestRepo != nil {
		if impact.Requests, err = s.requestRepo.CountByPhaseID(ctx, id); err != nil {
			return nil, fmt.Errorf("phase delete impact: count requests: %w", err)
		}
	}
	if s.careOfferingRepo != nil {
		if impact.CareOfferings, err = s.careOfferingRepo.CountByPhaseID(ctx, id); err != nil {
			return nil, fmt.Errorf("phase delete impact: count care offerings: %w", err)
		}
	}
	if s.requestChildRepo != nil {
		if impact.StudentsKept, err = s.requestChildRepo.CountCreatedStudentsByPhaseID(ctx, id); err != nil {
			return nil, fmt.Errorf("phase delete impact: count created students: %w", err)
		}
	}
	return impact, nil
}

// Delete permanently removes the phase and every enrollment record that
// hangs off it. There is intentionally no reference guard: admins may
// delete a phase at any lifecycle stage. To merely hide a phase from
// parents, use Update(is_active=false) instead.
//
// Ordering matters. enrollment.request_child_offerings.care_offering_id
// is an ON DELETE RESTRICT FK, so a single DELETE on phases (which would
// cascade requests and care_offerings concurrently) can fail. We delete
// requests first — that cascades request_children and
// request_child_offerings away, clearing the RESTRICT referrers — then
// delete the phase, whose cascade drops the now-unreferenced care
// offerings cleanly. Both steps run in one transaction.
//
// Students created from the phase are preserved: request_children
// .created_student_id is ON DELETE SET NULL and the student is the
// parent in that relationship, so deleting children never deletes the
// student.
func (s *phaseService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidPhase)
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return ErrPhaseNotFound
	}

	deletedRequests := 0
	run := func(txCtx context.Context) error {
		if s.requestRepo != nil {
			n, err := s.requestRepo.DeleteByPhaseID(txCtx, id)
			if err != nil {
				return fmt.Errorf("phase delete: requests: %w", err)
			}
			deletedRequests = n
		}
		if err := s.repo.Delete(txCtx, id); err != nil {
			return fmt.Errorf("phase delete: phase: %w", err)
		}
		return nil
	}

	var err error
	if s.txHandler != nil {
		err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
			return run(txCtx)
		})
	} else {
		// No DB wired (unit tests with mocks): run without an explicit
		// transaction. Ordering still holds; the caller's tenant tx, if
		// any, provides atomicity.
		err = run(ctx)
	}
	if err != nil {
		return err
	}

	s.logger.Info("phase deleted",
		slog.Int64("phase_id", id),
		slog.Int("deleted_requests", deletedRequests))
	return nil
}
