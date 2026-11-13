package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	// Resyncer keeps the rosters of the templates sourcing the phase's
	// offerings in step with a window change or a delete.
	Resyncer enrollment.SourcedTemplateResyncer
	// Students moves the enrollment window of still pending students with
	// a phase-start edit (#6505). Optional: nil leaves the students alone
	// (focused tests); the root always binds it.
	Students enrollment.PhaseStudents
	// Bookings carries the bounds the phase set on its offerings' bookings
	// with a service-window edit (#6597). Optional like Students; the root
	// always binds it.
	Bookings enrollment.PhaseBookings
	// CareStudents reads the care end of the phase's confirmed children, so
	// an edit that shortens the phase can name those whose care now runs
	// past it (#6598). Optional like Students; the root always binds it.
	CareStudents enrollment.PhaseCareStudents
	// Settings resolves the concrete-class collection toggles used to
	// reject unsatisfiable eligibility configs. Optional: nil skips the
	// guard (focused tests; the CHECK/model rules still apply).
	Settings CollectionSettings
	// Responses are the read ports of the response overview (#3379).
	// Optional: without them ResponseOverview reports
	// ErrPhaseResponseOverviewUnavailable.
	Responses *PhaseResponseSources
	Runtime   Runtime
	// TraegerCopies links the copies of Träger phases (ADR 0195). Optional:
	// without it TraegerCopies reports no copy commands.
	TraegerCopies TraegerPhaseCopyStore
	Logger        *slog.Logger
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
// Phases lists every phase straight from the records.
type Phases struct {
	PhaseList
	deps   PhaseDependencies
	logger *slog.Logger
}

// newPhases builds the phase administration. A nil logger falls back to
// slog.Default().
func newPhases(deps PhaseDependencies) *Phases {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Phases{PhaseList: deps.Records, deps: deps, logger: logger}
}

// TraegerCopies returns the copy commands of Träger phases, which write
// through this phase administration, or nil without their store.
func (s *Phases) TraegerCopies() enrollment.TraegerPhaseCopies {
	if s.deps.TraegerCopies == nil {
		return nil
	}
	return &TraegerPhaseCopies{phases: s, store: s.deps.TraegerCopies}
}

// NewResyncingPhases builds the phase administration over the
// sourced-template resyncer it keeps the rosters in step through. The
// records, the offerings, the recurrence lock and the resyncer are required.
func NewResyncingPhases(deps PhaseDependencies) (*Phases, error) {
	if deps.Records == nil || deps.Offerings == nil || deps.LockTemplateRecurrence == nil || deps.Resyncer == nil {
		return nil, errors.New("enrollment phases: records, offerings, template recurrence lock and sourced-template resyncer are required")
	}
	return newPhases(deps), nil
}

func (s *Phases) todayDate() calendar.Date {
	if s.deps.Today != nil {
		return s.deps.Today()
	}
	return calendar.TodayDate()
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
		return fmt.Errorf("%w: %w", enrollment.ErrInvalidPhase, err)
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
//
// It reports the confirmed children whose care runs past a shortened end
// (#6598) and changes none of them.
func (s *Phases) UpdatePhase(ctx context.Context, phase *enrollment.Phase) (enrollment.PhaseUpdateReport, error) {
	var report enrollment.PhaseUpdateReport
	err := s.runInTx(ctx, func(txCtx context.Context) error {
		var err error
		report, err = s.update(txCtx, phase)
		return err
	})
	if err != nil {
		return enrollment.PhaseUpdateReport{}, err
	}
	return report, nil
}

func (s *Phases) update(ctx context.Context, phase *enrollment.Phase) (enrollment.PhaseUpdateReport, error) {
	var report enrollment.PhaseUpdateReport
	if phase == nil || phase.ID <= 0 {
		return report, fmt.Errorf("%w: phase with valid id is required", enrollment.ErrInvalidPhase)
	}
	if err := s.validatePhaseWrite(ctx, phase); err != nil {
		return report, err
	}
	existing, serviceWindowChanged, err := s.validateCareOfferingPhaseUpdate(ctx, phase)
	if err != nil {
		return report, err
	}
	if err := s.deps.Records.UpdatePhase(ctx, phase); err != nil {
		return report, s.translatePhaseWriteError(err)
	}
	if serviceWindowChanged {
		if err := s.followBookingBounds(ctx, existing, phase); err != nil {
			return report, err
		}
		if err := s.followPendingActivations(ctx, existing, phase); err != nil {
			return report, err
		}
		if err := s.resyncPhaseSourcedTemplates(ctx, phase.ID); err != nil {
			return report, err
		}
		if report.CareBeyondEnd, err = s.careBeyondShortenedEnd(ctx, existing, phase); err != nil {
			return report, err
		}
	}
	s.logger.Info("phase updated",
		slog.Int64("phase_id", phase.ID),
		slog.Int("care_beyond_end", len(report.CareBeyondEnd)))
	return report, nil
}

// validateCareOfferingPhaseUpdate guards a service-window change against the
// legacy-linked care offerings and reports whether the window actually
// changed, so Update can resync offering-sourced templates after the write
// (#2147 review). The recurrence lock is taken before the existing row is
// read and stays held for the whole update transaction. It returns the
// stored phase, so the update can tell where its window was.
func (s *Phases) validateCareOfferingPhaseUpdate(ctx context.Context, phase *enrollment.Phase) (*enrollment.Phase, bool, error) {
	locked, err := enrollment.NewPhaseBoundsReader(s.deps.Records, s.deps.LockTemplateRecurrence).LockedPhase(ctx, phase.ID)
	if errors.Is(err, enrollment.ErrPhaseBoundsLock) {
		return nil, false, fmt.Errorf("lock template recurrence for phase update: %w", err)
	}
	existing := locked.Phase()
	if err != nil {
		if s.notFound(err) {
			return nil, false, fmt.Errorf("%w: phase %d", enrollment.ErrPhaseNotFound, phase.ID)
		}
		return nil, false, fmt.Errorf("load phase for care-offering validation: %w", err)
	}
	if existing.ServiceStartDate == phase.ServiceStartDate && existing.ServiceEndDate == phase.ServiceEndDate {
		return existing, false, nil
	}
	if s.deps.ValidateCareOfferingPhaseChange != nil {
		if err := s.deps.ValidateCareOfferingPhaseChange(ctx, phase.ID, phase); err != nil {
			if errors.Is(err, careplan.ErrCareOfferingConfigInvalid) {
				return nil, false, fmt.Errorf("%w: %v", enrollment.ErrPhaseCareOfferingConflict, err)
			}
			return nil, false, fmt.Errorf("validate care offerings for phase update: %w", err)
		}
	}
	return existing, true, nil
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
	offeringIDs, err := s.deps.Offerings.OfferingIDsForPhase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("phase update: list care offerings for sourced-template resync: %w", err)
	}
	if len(offeringIDs) == 0 {
		return nil
	}
	resyncer, ok := s.deps.Resyncer.(interface {
		ResyncTemplatesSourcedFromOfferings(context.Context, []int64, calendar.Date) error
	})
	if !ok {
		return errors.New("phase update: batch sourced-template resync is required")
	}
	if err := resyncer.ResyncTemplatesSourcedFromOfferings(ctx, offeringIDs, s.todayDate()); err != nil {
		if errors.Is(err, timetable.ErrOfferingSourceInvalid) {
			// The tenant transaction commits ordinary 4xx responses.
			// Mark the ambient transaction so the already-written phase
			// update is discarded together with the rejection.
			if s.deps.Runtime.MarkRollback != nil {
				s.deps.Runtime.MarkRollback(ctx)
			}
			return fmt.Errorf("%w: %w", enrollment.ErrPhaseCareOfferingConflict, err)
		}
		return fmt.Errorf("phase update: resync templates sourcing offerings %v: %w", offeringIDs, err)
	}
	return nil
}

// detachPhaseSourcedTemplates retires the sourced rosters of every template
// sourcing one of the phase's offerings ahead of the phase delete (#2147
// review round 11). Runs inside the delete transaction under the tenant
// recurrence lock. Deliberately no source validation is involved, so a
// drifted-invalid source never blocks the delete.
func (s *Phases) detachPhaseSourcedTemplates(ctx context.Context, phaseID int64) error {
	if err := s.deps.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("lock template recurrence for phase delete: %w", err)
	}
	offeringIDs, err := s.deps.Offerings.OfferingIDsForPhase(ctx, phaseID)
	if err != nil {
		return fmt.Errorf("phase delete: list care offerings for sourced-template detach: %w", err)
	}
	today := s.todayDate()
	for _, offeringID := range offeringIDs {
		if err := s.deps.Resyncer.DetachTemplatesSourcedFromOffering(ctx, offeringID, today); err != nil {
			return fmt.Errorf("phase delete: detach templates sourcing offering %d: %w", offeringID, err)
		}
	}
	return nil
}

// DeleteImpact returns what deleting the phase would touch so the admin UI
// can warn before the destructive action. Requests block the delete
// (#6836); CareOfferings are deleted with the phase; StudentsKept counts the
// students created from its requests.
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

// DeletePhase removes a phase that holds no enrollment requests, together
// with its care offerings. A phase with requests is refused before any write
// (#6836): requests, guardians, offering choices, uploads and the offering
// adjustment audit are evidence of application, decision and consent, and
// leave only through the audited request deletion or retention. To retire
// such a phase, deactivate it (Update(is_active=false)), as the Träger
// withdrawal does (ADR 0195 section 7). A phase whose care offering bookings
// still apply today or later is refused first with its own reason (#6505).
func (s *Phases) DeletePhase(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", enrollment.ErrInvalidPhase)
	}
	if s.deps.Records == nil {
		return errors.New("phase delete requires the enrollment capability")
	}
	phase, err := s.deps.Records.Phase(ctx, id)
	if err != nil {
		if s.notFound(err) {
			return enrollment.ErrPhaseNotFound
		}
		return fmt.Errorf("phase delete: find phase: %w", err)
	}
	// Only the Träger office withdraws a copy (ADR 0195 section 7); the
	// next rollout would otherwise create it again.
	if phase.Traeger != nil {
		return enrollment.ErrTraegerCopyDelete
	}

	err = s.runInTx(ctx, func(txCtx context.Context) error {
		// The recurrence gate serializes the check with the approvals and
		// booking changes that write bookings under it.
		if err := s.deps.LockTemplateRecurrence(txCtx); err != nil {
			return fmt.Errorf("lock template recurrence for phase delete: %w", err)
		}
		if err := s.refuseDelete(txCtx, id); err != nil {
			return err
		}
		// Retire sourced rosters FIRST, while offering links still exist:
		// the phase cascade SET-NULLs the templates' source and the rows'
		// provenance, which would leave bounded offering-derived enrollment
		// rows that keep materializing children yet are invisible to every
		// later resync (#2147 review round 11).
		if err := s.detachPhaseSourcedTemplates(txCtx, id); err != nil {
			return err
		}
		return s.deps.Records.RemovePhase(txCtx, id)
	})
	if err != nil {
		return err
	}

	s.logger.Info("phase deleted", slog.Int64("phase_id", id))
	return nil
}

// refuseDelete runs the refusals of a phase delete before any write: Träger
// copies, bookings that still apply (#6505), and requests (#6836).
func (s *Phases) refuseDelete(ctx context.Context, id int64) error {
	if err := s.refuseTraegerOfferings(ctx, id); err != nil {
		return err
	}
	if err := s.refuseEffectiveBookings(ctx, id); err != nil {
		return err
	}
	return s.refuseRequests(ctx, id)
}

// refuseRequests rejects deleting a phase that holds enrollment requests
// (#6836). The owner command repeats the check as its own guard; this one
// answers before the sourced templates are detached.
func (s *Phases) refuseRequests(ctx context.Context, id int64) error {
	requests, err := s.deps.Records.CountPhaseRequests(ctx, id)
	if err != nil {
		return fmt.Errorf("phase delete: count requests: %w", err)
	}
	if requests > 0 {
		return fmt.Errorf("%w: %d requests", enrollment.ErrPhaseHasRequests, requests)
	}
	return nil
}

func (s *Phases) refuseTraegerOfferings(ctx context.Context, id int64) error {
	phase, err := s.deps.Records.Phase(ctx, id)
	if err != nil {
		return err
	}
	if phase.Traeger != nil {
		return enrollment.ErrTraegerCopyDelete
	}
	if s.deps.Offerings == nil {
		return errors.New("phase delete requires the offering guard")
	}
	copies, err := s.deps.Offerings.HasTraegerCopiesForPhase(ctx, id)
	if err != nil {
		return err
	}
	if copies {
		return enrollment.ErrTraegerCopyDelete
	}
	return nil
}
