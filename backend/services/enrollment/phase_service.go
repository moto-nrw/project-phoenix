package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// PhaseSettingsResolver is the narrow slice of the settings service the
// phase service needs: the two toggles that decide whether the enrollment
// form collects a concrete school class, plus the tenant's grade-level cap.
// A class-based eligibility restriction (eligible_school_classes) is
// uncheckable — and every submission is rejected — when the class is never
// collected, and a grade restriction above enrollment.grade_level_max is
// unsatisfiable for the same reason (the form never offers that grade), so
// Create/Update reject both configurations up front. Optional: when nil
// (unit tests with mocks) the collectability guard is skipped.
type PhaseSettingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// PhaseService sentinel errors. The HTTP layer maps these to status
// codes; tests assert on them via errors.Is.
var (
	ErrPhaseNotFound             = errors.New("phase not found")
	ErrInvalidPhase              = errors.New("invalid phase")
	ErrPhaseDuplicateName        = errors.New("phase name already exists")
	ErrPhaseCareOfferingConflict = errors.New("phase change is incompatible with a linked care offering")
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
	FormSchemaRepo   enrollmentModels.FormSchemaRepository
	// CalendarPeriods validates phase→calendar-period links on
	// Create/Update. Optional: when nil (unit tests with mocks), the
	// link is accepted unvalidated and the FK constraint still holds.
	CalendarPeriods                 scheduleService.CalendarPeriodService
	LockTemplateRecurrence          func(context.Context) error
	ValidateCareOfferingPhaseChange func(context.Context, int64, *enrollmentModels.Phase) error
	// Settings resolves the concrete-class collection toggles used to
	// reject unsatisfiable eligibility configs. Optional: nil skips the
	// guard (unit tests with mocks; the CHECK/model rules still apply).
	Settings PhaseSettingsResolver
	DB       *bun.DB
	Logger   *slog.Logger
}

type phaseService struct {
	repo                            enrollmentModels.PhaseRepository
	requestRepo                     enrollmentModels.RequestRepository
	requestChildRepo                enrollmentModels.RequestChildRepository
	careOfferingRepo                enrollmentModels.CareOfferingRepository
	formSchemaRepo                  enrollmentModels.FormSchemaRepository
	calendarPeriods                 scheduleService.CalendarPeriodService
	lockTemplateRecurrence          func(context.Context) error
	validateCareOfferingPhaseChange func(context.Context, int64, *enrollmentModels.Phase) error
	settings                        PhaseSettingsResolver
	txHandler                       *modelBase.TxHandler
	logger                          *slog.Logger
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
		repo:                            cfg.Repo,
		requestRepo:                     cfg.RequestRepo,
		requestChildRepo:                cfg.RequestChildRepo,
		careOfferingRepo:                cfg.CareOfferingRepo,
		formSchemaRepo:                  cfg.FormSchemaRepo,
		calendarPeriods:                 cfg.CalendarPeriods,
		lockTemplateRecurrence:          cfg.LockTemplateRecurrence,
		validateCareOfferingPhaseChange: cfg.ValidateCareOfferingPhaseChange,
		settings:                        cfg.Settings,
		txHandler:                       txHandler,
		logger:                          logger,
	}
}

// classCollectionResolver is the minimal settings surface the collectability
// guard reads: the two collection toggles plus enrollment.grade_level_max for
// the grade-cap check. The concrete settings service additionally satisfies the
// optional LockClassCollectionPair interface the guard type-asserts for.
type classCollectionResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// ensureEligibleClassesCollectable rejects a class-based eligibility
// restriction the school cannot actually collect. eligible_school_classes
// is checked at submit against each child's declared concrete class, but
// the form only collects that class when BOTH collect_grade_level and
// collect_school_class are on (a class without its grade is ambiguous).
// With collection off, validateAndNormalizeSchoolClasses forces every
// child's class to nil, so a non-empty eligibility list rejects every
// submission with class_not_eligible. Reject the config here so the admin
// enables class collection (or clears the list) first (#1663). Skipped
// when no settings resolver is wired. Shared by phaseService.Create/Update
// and rolloverService (an inactive class-restricted source must not be rolled
// into an active successor while collection is off).
func ensureEligibleClassesCollectable(ctx context.Context, settings classCollectionResolver, eligible []string) error {
	if settings == nil || !hasNonEmptyEligibleClass(eligible) {
		return nil
	}
	// Serialize against a concurrent settings write disabling concrete-class
	// collection: services/config validateClassCollectionGuard takes the same
	// per-tenant lock, so the two invariant sides can't both pass on a stale
	// read and commit an active restricted phase with collection off (#1663).
	// The concrete settings service implements the lock; minimal test fakes
	// don't, so it degrades to best-effort there.
	if locker, ok := settings.(interface {
		LockClassCollectionPair(context.Context) error
	}); ok {
		if err := locker.LockClassCollectionPair(ctx); err != nil {
			return fmt.Errorf("lock class-collection pair: %w", err)
		}
	}
	collectGrade, err := settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
	}
	collectClass, err := settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectSchoolClass, err)
	}
	if !collectGrade || !collectClass {
		return fmt.Errorf("%w: eligible_school_classes requires the concrete-class collection settings (Klassen-Abfrage) to be active", ErrInvalidPhase)
	}
	return nil
}

// ensureEligibleGradeLevelsCollectable is the grade-level counterpart of
// ensureEligibleClassesCollectable. A grade restriction is checked at submit
// against each child's declared grade level, which the form collects only
// while collect_grade_level is on — with it off,
// normalizeSubmissionForCapabilities nils every child's grade and the
// restriction rejects every submission with grade_not_eligible. Note the
// weaker requirement than the class guard: a whole-grade phase needs the grade
// toggle ALONE, which is exactly why enumerating concrete classes is not a
// substitute for it (#1663). Skipped when no settings resolver is wired.
func ensureEligibleGradeLevelsCollectable(ctx context.Context, settings classCollectionResolver, eligible []int) error {
	if settings == nil || len(eligible) == 0 {
		return nil
	}
	// Same per-tenant lock as the class guard: the settings side takes it
	// before deciding whether collect_grade_level may be turned off, so the two
	// invariant sides cannot both pass on a stale read (#1663).
	if locker, ok := settings.(interface {
		LockClassCollectionPair(context.Context) error
	}); ok {
		if err := locker.LockClassCollectionPair(ctx); err != nil {
			return fmt.Errorf("lock class-collection pair: %w", err)
		}
	}
	collectGrade, err := settings.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentCollectGradeLevel, err)
	}
	if !collectGrade {
		return fmt.Errorf("%w: eligible_grade_levels requires the grade-level collection setting (Klassenstufen-Abfrage) to be active", ErrInvalidPhase)
	}
	return ensureEligibleGradeLevelsWithinTenantCap(ctx, settings, eligible)
}

// ensureEligibleGradeLevelsWithinTenantCap rejects a grade restriction the
// tenant's own form can never satisfy. The public form offers grades
// 1..enrollment.grade_level_max and the submit path re-checks the same cap, so
// a phase restricted to a grade ABOVE it is unsatisfiable from both ends: the
// select never offers the grade, and a hand-crafted submission carrying it is
// rejected by the cap before eligibility is even consulted. Without this check
// an admin can save eligible_grade_levels [5] under a cap of 4 and end up with
// an active phase that accepts no submission at all (#1663).
//
// Runs under the lock the caller already holds, which the settings side takes
// too — so lowering the cap and adding an above-cap restriction cannot both
// pass on a stale read. Values below MinGradeLevel are already rejected by
// Phase.Validate's normalizeGradeLevelList; the cap is the tenant-specific half
// it cannot know about.
func ensureEligibleGradeLevelsWithinTenantCap(ctx context.Context, settings classCollectionResolver, eligible []int) error {
	gradeMax, err := settings.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", configModel.KeyEnrollmentGradeLevelMax, err)
	}
	// A corrupt or out-of-range cap must stop the write rather than silently
	// pick a substitute bound — the same fail-closed reasoning as
	// requestService.resolveGradeMax and rolloverService.resolveMaxGrade.
	if gradeMax < schoolclass.MinGradeLevel || gradeMax > schoolclass.MaxGradeLevel {
		return fmt.Errorf(
			"resolve %s: value %d is outside %d..%d",
			configModel.KeyEnrollmentGradeLevelMax,
			gradeMax,
			schoolclass.MinGradeLevel,
			schoolclass.MaxGradeLevel,
		)
	}
	for _, level := range eligible {
		if level > gradeMax {
			return fmt.Errorf(
				"%w: eligible_grade_levels contains grade %d above the tenant maximum %d (Höchste Klassenstufe im Formular); the form never offers that grade, so every submission would be rejected",
				ErrInvalidPhase,
				level,
				gradeMax,
			)
		}
	}
	return nil
}

// validateEligibleClassesCollectable is the phaseService adapter over the two
// collectability guards used by Create/Update.
//
// Only ACTIVE phases are guarded. The invariant these guards protect is
// "no ACTIVE restricted phase while the matching collection setting is off" —
// exactly the scope the settings side enforces from its end
// (ExistsActiveWithEligibleClasses / ExistsActiveWithEligibleGradeLevels only
// look at is_active phases). Applying them to an inactive phase made a
// historical restricted phase unwritable once collection was disabled: a name
// or date correction, or even deactivating the phase, hit the guard and could
// not be saved at all, while clearing the restriction to get past it would
// destroy the record of what that phase was restricted to. Reactivating such a
// phase still goes through Update with is_active=true and is still refused
// (#1663).
func (s *phaseService) validateEligibleClassesCollectable(ctx context.Context, phase *enrollmentModels.Phase) error {
	if s.settings == nil || !phase.IsActive {
		return nil
	}
	if err := ensureEligibleGradeLevelsCollectable(ctx, s.settings, phase.EligibleGradeLevels); err != nil {
		return err
	}
	return ensureEligibleClassesCollectable(ctx, s.settings, phase.EligibleSchoolClasses)
}

func hasNonEmptyEligibleClass(classes []string) bool {
	for _, c := range classes {
		if strings.TrimSpace(c) != "" {
			return true
		}
	}
	return false
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

func (s *phaseService) validateFormSchemaLink(ctx context.Context, phase *enrollmentModels.Phase) error {
	if phase.FormSchemaID == nil || s.formSchemaRepo == nil {
		return nil
	}
	if _, err := s.formSchemaRepo.FindByID(ctx, *phase.FormSchemaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: form schema %d not found", ErrInvalidPhase, *phase.FormSchemaID)
		}
		return fmt.Errorf("validate form schema link: %w", err)
	}
	return nil
}

func translatePhaseWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case isPhaseDuplicateName(err):
		return fmt.Errorf("%w: %v", ErrPhaseDuplicateName, err)
	case errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("%w: %v", ErrPhaseNotFound, err)
	case isPhaseReferenceViolation(err):
		return fmt.Errorf("%w: %v", ErrInvalidPhase, err)
	default:
		return err
	}
}

func isPhaseReferenceViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr pgdriver.Error
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Field('C') != "23503" {
		return false
	}
	switch pgErr.Field('n') {
	case "phases_form_schema_id_fkey", "phases_calendar_period_id_fkey":
		return true
	default:
		return false
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPhaseNotFound
		}
		return nil, fmt.Errorf("get phase %d: %w", id, err)
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
	if err := s.validateEligibleClassesCollectable(ctx, phase); err != nil {
		return nil, err
	}
	if err := s.validateCalendarPeriodLink(ctx, phase); err != nil {
		return nil, err
	}
	if err := s.validateFormSchemaLink(ctx, phase); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, phase); err != nil {
		return nil, translatePhaseWriteError(err)
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
	if err := s.validateEligibleClassesCollectable(ctx, phase); err != nil {
		return err
	}
	if err := s.validateCalendarPeriodLink(ctx, phase); err != nil {
		return err
	}
	if err := s.validateFormSchemaLink(ctx, phase); err != nil {
		return err
	}
	if err := s.validateCareOfferingPhaseUpdate(ctx, phase); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, phase); err != nil {
		return translatePhaseWriteError(err)
	}
	s.logger.Info("phase updated", slog.Int64("phase_id", phase.ID))
	return nil
}

func (s *phaseService) validateCareOfferingPhaseUpdate(ctx context.Context, phase *enrollmentModels.Phase) error {
	if s.validateCareOfferingPhaseChange == nil {
		return nil
	}
	if s.lockTemplateRecurrence == nil {
		return errors.New("phase update care-offering validation requires the template recurrence lock")
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("lock template recurrence for phase update: %w", err)
	}
	existing, err := s.repo.FindByID(ctx, phase.ID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return fmt.Errorf("%w: phase %d", ErrPhaseNotFound, phase.ID)
		}
		return fmt.Errorf("load phase for care-offering validation: %w", err)
	}
	if existing.ServiceStartDate == phase.ServiceStartDate &&
		existing.ServiceEndDate == phase.ServiceEndDate {
		return nil
	}
	if err := s.validateCareOfferingPhaseChange(ctx, phase.ID, phase); err != nil {
		if errors.Is(err, ErrCareOfferingInvalid) {
			return fmt.Errorf("%w: %v", ErrPhaseCareOfferingConflict, err)
		}
		return fmt.Errorf("validate care offerings for phase update: %w", err)
	}
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPhaseNotFound
		}
		return nil, fmt.Errorf("phase delete impact: find phase: %w", err)
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
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPhaseNotFound
		}
		return fmt.Errorf("phase delete: find phase: %w", err)
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
