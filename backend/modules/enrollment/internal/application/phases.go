package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PhaseDependencies bind the phase administration to its owners.
type PhaseDependencies struct {
	Records   PhaseRecords
	Offerings PhaseOfferings
	// Calendar validates phase→calendar-period links on Create/Update
	// through the School Calendar. Optional: when nil (focused tests), the
	// link is accepted unvalidated and the FK constraint still holds.
	Calendar                        CalendarPeriods
	LockTemplateRecurrence          func(context.Context) error
	ValidateCareOfferingPhaseChange func(context.Context, int64, *enrollment.Phase) error
	// SourcedTemplates resolves the decision flow's sourced-template
	// resyncer on every edit, so the composition can bind it after the
	// phase administration exists.
	SourcedTemplates func() enrollment.SourcedTemplateResyncer
	// Settings resolves the concrete-class collection toggles used to
	// reject unsatisfiable eligibility configs. Optional: nil skips the
	// guard (focused tests; the CHECK/model rules still apply).
	Settings CollectionSettings
	// Responses are the read ports of the response overview (#3379).
	// Optional: without them ResponseOverview reports
	// ErrPhaseResponseOverviewUnavailable.
	Responses *PhaseResponseSources
	Runtime   Runtime
	Logger    *slog.Logger
	// Today returns the current calendar day; tests inject a fixed date so
	// resync/detach boundaries stay deterministic. Nil falls back to
	// calendar.TodayDate.
	Today func() calendar.Date
}

// PhaseResponseSources bundles the non-owner read ports of the overview.
type PhaseResponseSources struct {
	Roster         enrollment.PhaseResponseRoster
	CareExits      enrollment.PhaseResponseCareExits
	PortalAccounts enrollment.PhaseResponsePortalAccounts
}

func (s *PhaseResponseSources) complete() bool {
	return s != nil && s.Roster != nil && s.CareExits != nil && s.PortalAccounts != nil
}

// Phases is the phase administration.
type Phases struct {
	deps   PhaseDependencies
	logger *slog.Logger
}

// NewPhases builds the phase administration. A nil logger falls back to
// slog.Default().
func NewPhases(deps PhaseDependencies) *Phases {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Phases{deps: deps, logger: logger}
}

func (s *Phases) todayDate() calendar.Date {
	if s.deps.Today != nil {
		return s.deps.Today()
	}
	return calendar.TodayDate()
}

func (s *Phases) sourcedTemplates() enrollment.SourcedTemplateResyncer {
	if s.deps.SourcedTemplates == nil {
		return nil
	}
	return s.deps.SourcedTemplates()
}

func (s *Phases) notFound(err error) bool {
	return s.deps.Runtime.NotFound != nil && s.deps.Runtime.NotFound(err)
}

// runInTx runs fn in the caller's tenant transaction, or opens one. Focused
// tests without transactions run fn directly; ordering still holds.
func (s *Phases) runInTx(ctx context.Context, fn func(context.Context) error) error {
	if s.deps.Runtime.Transactions == nil {
		return fn(ctx)
	}
	return s.deps.Runtime.Transactions.RunInTx(ctx, fn)
}

// validateCalendarPeriodLink checks that a linked calendar period exists
// for the current tenant. The lookup runs inside the tenant transaction,
// so RLS scopes it — this is what actually blocks cross-tenant links,
// because FK constraint checks bypass RLS.
func (s *Phases) validateCalendarPeriodLink(ctx context.Context, phase *enrollment.Phase) error {
	if phase.CalendarPeriodID == nil || s.deps.Calendar == nil {
		return nil
	}
	if _, err := s.deps.Calendar.FindCalendarPeriod(ctx, *phase.CalendarPeriodID); err != nil {
		if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
			return fmt.Errorf("%w: calendar period %d not found", enrollment.ErrInvalidPhase, *phase.CalendarPeriodID)
		}
		return fmt.Errorf("validate calendar period link: %w", err)
	}
	return nil
}

func (s *Phases) validateFormSchemaLink(ctx context.Context, phase *enrollment.Phase) error {
	if phase.FormSchemaID == nil || s.deps.Records == nil {
		return nil
	}
	if _, err := s.deps.Records.Schema(ctx, *phase.FormSchemaID); err != nil {
		if s.notFound(err) {
			return fmt.Errorf("%w: form schema %d not found", enrollment.ErrInvalidPhase, *phase.FormSchemaID)
		}
		return fmt.Errorf("validate form schema link: %w", err)
	}
	return nil
}

func (s *Phases) translatePhaseWriteError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, enrollment.ErrPhaseNameTaken):
		return fmt.Errorf("%w: %v", enrollment.ErrPhaseDuplicateName, err)
	case s.notFound(err):
		return fmt.Errorf("%w: %v", enrollment.ErrPhaseNotFound, err)
	case errors.Is(err, enrollment.ErrPhaseReferenceMissing):
		return fmt.Errorf("%w: %v", enrollment.ErrInvalidPhase, err)
	default:
		return err
	}
}

// AllPhases returns every phase of the tenant.
func (s *Phases) AllPhases(ctx context.Context) ([]*enrollment.Phase, error) {
	return s.deps.Records.Phases(ctx)
}

// ListPublicOpen returns the active phases whose enrollment window is open
// at now; a zero now means the current instant.
func (s *Phases) ListPublicOpen(ctx context.Context, now time.Time) ([]*enrollment.Phase, error) {
	if now.IsZero() {
		now = time.Now()
	}
	return s.deps.Records.PublicOpenPhases(ctx, now)
}

// PhaseByID returns one phase or ErrPhaseNotFound.
func (s *Phases) PhaseByID(ctx context.Context, id int64) (*enrollment.Phase, error) {
	if id <= 0 {
		return nil, enrollment.ErrPhaseNotFound
	}
	phase, err := s.deps.Records.Phase(ctx, id)
	if err != nil {
		if s.notFound(err) {
			return nil, enrollment.ErrPhaseNotFound
		}
		return nil, fmt.Errorf("get phase %d: %w", id, err)
	}
	return phase, nil
}

// CreatePhase validates and inserts a phase.
func (s *Phases) CreatePhase(ctx context.Context, phase *enrollment.Phase) (*enrollment.Phase, error) {
	var result *enrollment.Phase
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.create(txCtx, phase)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Phases) validatePhaseWrite(ctx context.Context, phase *enrollment.Phase) error {
	if err := phase.Validate(); err != nil {
		return fmt.Errorf("%w: %v", enrollment.ErrInvalidPhase, err)
	}
	if err := s.validateEligibleClassesCollectable(ctx, phase); err != nil {
		return err
	}
	if err := s.validateCalendarPeriodLink(ctx, phase); err != nil {
		return err
	}
	return s.validateFormSchemaLink(ctx, phase)
}

func (s *Phases) create(ctx context.Context, phase *enrollment.Phase) (*enrollment.Phase, error) {
	if phase == nil {
		return nil, fmt.Errorf("%w: phase is required", enrollment.ErrInvalidPhase)
	}
	if err := s.validatePhaseWrite(ctx, phase); err != nil {
		return nil, err
	}
	if err := s.deps.Records.InsertPhase(ctx, phase); err != nil {
		return nil, s.translatePhaseWriteError(err)
	}
	s.logger.Info("phase created",
		slog.Int64("phase_id", phase.ID),
		slog.String("name", phase.Name),
		slog.String("kind", phase.Kind))
	return phase, nil
}

// UpdatePhase validates and writes a phase and resyncs the templates sourcing its
// offerings when the service window changed.
func (s *Phases) UpdatePhase(ctx context.Context, phase *enrollment.Phase) error {
	return s.runInTx(ctx, func(txCtx context.Context) error {
		return s.update(txCtx, phase)
	})
}

func (s *Phases) update(ctx context.Context, phase *enrollment.Phase) error {
	if phase == nil || phase.ID <= 0 {
		return fmt.Errorf("%w: phase with valid id is required", enrollment.ErrInvalidPhase)
	}
	if err := s.validatePhaseWrite(ctx, phase); err != nil {
		return err
	}
	serviceWindowChanged, err := s.validateCareOfferingPhaseUpdate(ctx, phase)
	if err != nil {
		return err
	}
	if err := s.deps.Records.UpdatePhase(ctx, phase); err != nil {
		return s.translatePhaseWriteError(err)
	}
	if serviceWindowChanged {
		if err := s.resyncPhaseSourcedTemplates(ctx, phase.ID); err != nil {
			return err
		}
	}
	s.logger.Info("phase updated", slog.Int64("phase_id", phase.ID))
	return nil
}

// validateCareOfferingPhaseUpdate guards a service-window change against the
// legacy-linked care offerings and reports whether the window actually
// changed, so Update can resync offering-sourced templates after the write
// (#2147 review). The recurrence lock is taken before the existing row is
// read and stays held for the whole update transaction.
func (s *Phases) validateCareOfferingPhaseUpdate(ctx context.Context, phase *enrollment.Phase) (bool, error) {
	if s.deps.ValidateCareOfferingPhaseChange == nil && s.sourcedTemplates() == nil {
		return false, nil
	}
	if s.deps.LockTemplateRecurrence == nil {
		return false, errors.New("phase update care-offering validation requires the template recurrence lock")
	}
	if err := s.deps.LockTemplateRecurrence(ctx); err != nil {
		return false, fmt.Errorf("lock template recurrence for phase update: %w", err)
	}
	existing, err := s.deps.Records.Phase(ctx, phase.ID)
	if err != nil {
		if s.notFound(err) {
			return false, fmt.Errorf("%w: phase %d", enrollment.ErrPhaseNotFound, phase.ID)
		}
		return false, fmt.Errorf("load phase for care-offering validation: %w", err)
	}
	if existing.ServiceStartDate == phase.ServiceStartDate && existing.ServiceEndDate == phase.ServiceEndDate {
		return false, nil
	}
	if s.deps.ValidateCareOfferingPhaseChange != nil {
		if err := s.deps.ValidateCareOfferingPhaseChange(ctx, phase.ID, phase); err != nil {
			if errors.Is(err, careplan.ErrCareOfferingConfigInvalid) {
				return false, fmt.Errorf("%w: %v", enrollment.ErrPhaseCareOfferingConflict, err)
			}
			return false, fmt.Errorf("validate care offerings for phase update: %w", err)
		}
	}
	return true, nil
}

// resyncPhaseSourcedTemplates re-reconciles every template sourcing one of
// the phase's offerings after a service-window change (#2147 review): the
// window bounds every sourced enrollment row and the reconciled materialized
// occurrences, so extensions and shortenings must propagate immediately, not
// at the next unrelated template save. A window change that makes a source
// incompatible with its template's planning period is rejected as
// ErrPhaseCareOfferingConflict (#2147 review round 7), mirroring the
// care-offering update path: committing it would leave that template's
// sourced rows and materialized occurrences stranded, with every later resync
// skipping the template. Runs under the recurrence lock the update already
// holds; history before today stays untouched.
func (s *Phases) resyncPhaseSourcedTemplates(ctx context.Context, phaseID int64) error {
	resyncer := s.sourcedTemplates()
	if resyncer == nil {
		// Focused tests may run without the enrollment decision wiring; the
		// composition always binds it.
		s.logger.Warn("phase update: sourced-template resyncer not configured; sourced rosters may be stale",
			slog.Int64("phase_id", phaseID))
		return nil
	}
	offeringIDs, err := s.deps.Offerings.OfferingIDsForPhase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("phase update: list care offerings for sourced-template resync: %w", err)
	}
	today := s.todayDate()
	for _, offeringID := range offeringIDs {
		if err := resyncer.ResyncTemplatesSourcedFromOffering(ctx, offeringID, today); err != nil {
			if errors.Is(err, timetable.ErrOfferingSourceInvalid) {
				// The tenant transaction commits ordinary 4xx responses.
				// Mark the ambient transaction so the already-written phase
				// update is discarded together with the rejection.
				if s.deps.Runtime.MarkRollback != nil {
					s.deps.Runtime.MarkRollback(ctx)
				}
				return fmt.Errorf("%w: %w", enrollment.ErrPhaseCareOfferingConflict, err)
			}
			return fmt.Errorf("phase update: resync templates sourcing offering %d: %w", offeringID, err)
		}
	}
	return nil
}

// detachPhaseSourcedTemplates retires the sourced rosters of every template
// sourcing one of the phase's offerings ahead of the phase delete (#2147
// review round 11). Runs inside the delete transaction under the tenant
// recurrence lock. Deliberately no source validation is involved, so a
// drifted-invalid source never blocks the delete.
func (s *Phases) detachPhaseSourcedTemplates(ctx context.Context, phaseID int64) error {
	resyncer := s.sourcedTemplates()
	if resyncer == nil || s.deps.Offerings == nil {
		// Focused tests may run without the enrollment decision wiring; the
		// composition always binds it.
		s.logger.Warn("phase delete: sourced-template resyncer not configured; sourced rosters may be orphaned",
			slog.Int64("phase_id", phaseID))
		return nil
	}
	if s.deps.LockTemplateRecurrence == nil {
		return errors.New("phase delete sourced-template detach requires the template recurrence lock")
	}
	if err := s.deps.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("lock template recurrence for phase delete: %w", err)
	}
	offeringIDs, err := s.deps.Offerings.OfferingIDsForPhase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("phase delete: list care offerings for sourced-template detach: %w", err)
	}
	today := s.todayDate()
	for _, offeringID := range offeringIDs {
		if err := resyncer.DetachTemplatesSourcedFromOffering(ctx, offeringID, today); err != nil {
			return fmt.Errorf("phase delete: detach templates sourcing offering %d: %w", offeringID, err)
		}
	}
	return nil
}

// DeleteImpact returns the blast radius of deleting the phase so the
// admin UI can warn before the destructive action. Requests +
// CareOfferings are deleted; StudentsKept survive.
func (s *Phases) DeleteImpact(ctx context.Context, id int64) (*enrollment.PhaseDeleteImpact, error) {
	if id <= 0 {
		return nil, enrollment.ErrPhaseNotFound
	}
	if _, err := s.deps.Records.Phase(ctx, id); err != nil {
		if s.notFound(err) {
			return nil, enrollment.ErrPhaseNotFound
		}
		return nil, fmt.Errorf("phase delete impact: find phase: %w", err)
	}

	impact := &enrollment.PhaseDeleteImpact{}
	var err error
	if impact.Requests, err = s.deps.Records.CountPhaseRequests(ctx, id); err != nil {
		return nil, fmt.Errorf("phase delete impact: count requests: %w", err)
	}
	if s.deps.Offerings != nil {
		if impact.CareOfferings, err = s.deps.Offerings.CountOfferingsForPhase(ctx, id); err != nil {
			return nil, fmt.Errorf("phase delete impact: count care offerings: %w", err)
		}
	}
	if impact.StudentsKept, err = s.deps.Records.CountCreatedStudentsByPhase(ctx, id); err != nil {
		return nil, fmt.Errorf("phase delete impact: count created students: %w", err)
	}
	return impact, nil
}

// DeletePhase permanently removes the phase and every enrollment record that
// hangs off it. There is intentionally no reference guard: admins may
// delete a phase at any lifecycle stage. To merely hide a phase from
// parents, use Update(is_active=false) instead.
//
// Ordering matters. The care_offering_id FKs of care_offering_bookings and
// request_child_offering_selections are ON DELETE RESTRICT, so a single
// DELETE on phases (which would cascade requests and care_offerings
// concurrently) can fail. The owner deletes requests first — that cascades
// request_children, bookings and selections away, clearing the RESTRICT
// referrers — then the phase, whose cascade drops the now-unreferenced care
// offerings cleanly. Both steps run in one transaction.
//
// Students created from the phase are preserved: request_children
// .created_student_id is ON DELETE SET NULL and the student is the
// parent in that relationship, so deleting children never deletes the
// student.
func (s *Phases) DeletePhase(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", enrollment.ErrInvalidPhase)
	}
	if s.deps.Records == nil {
		return errors.New("phase delete requires the enrollment capability")
	}
	if _, err := s.deps.Records.Phase(ctx, id); err != nil {
		if s.notFound(err) {
			return enrollment.ErrPhaseNotFound
		}
		return fmt.Errorf("phase delete: find phase: %w", err)
	}

	deletedRequests := 0
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		// Retire sourced rosters FIRST, while request children and offering
		// links still exist: the phase cascade SET-NULLs the templates'
		// source and the rows' provenance, which would leave bounded
		// offering-derived enrollment rows that keep materializing children
		// yet are invisible to every later resync (#2147 review round 11).
		if err := s.detachPhaseSourcedTemplates(txCtx, id); err != nil {
			return err
		}
		var err error
		deletedRequests, err = s.deps.Records.RemovePhase(txCtx, id)
		return err
	})
	if err != nil {
		return err
	}

	s.logger.Info("phase deleted",
		slog.Int64("phase_id", id),
		slog.Int("deleted_requests", deletedRequests))
	return nil
}
