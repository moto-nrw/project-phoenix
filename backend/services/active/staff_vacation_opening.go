package active

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Sentinel errors for handler mapping (#2132).
var (
	// ErrVacationOpeningInvalid marks a caller mistake in a vacation opening
	// request — HTTP 400.
	ErrVacationOpeningInvalid = errors.New("invalid vacation opening")
	// ErrVacationOpeningNotFound marks a delete for a staff/year without an
	// opening row — HTTP 404.
	ErrVacationOpeningNotFound = errors.New("vacation opening not found")
	// ErrVacationOpeningExists marks a second takeover for the same staff
	// member and year — HTTP 409. Corrections are delete + re-create so the
	// deletion tombstone keeps the history.
	ErrVacationOpeningExists = errors.New("vacation opening already exists for this year")
	// ErrVacationOpeningAbsencesBeforeCutoff rejects a takeover when vacation
	// absences already exist before the Stichtag: their days are part of the
	// old system's Resturlaub figure AND would be subtracted again by the
	// in-app arithmetic — a silent double count — HTTP 409.
	ErrVacationOpeningAbsencesBeforeCutoff = errors.New("vacation absences exist before the opening date")
)

// vacationOpeningUniqueConstraintName guards one opening per staff and year
// (migration 1.15.257).
const vacationOpeningUniqueConstraintName = "uq_svo_staff_year"

// SetVacationOpeningRequest carries the vacation takeover (#2132). The admin
// enters the Resturlaub as of the Stichtag; the service derives the days
// taken before the introduction from entitled + carryover − remaining, so the
// Jahresanspruch stays authoritative and later entitlement corrections shift
// the Resturlaub coherently.
type SetVacationOpeningRequest struct {
	EffectiveDate timezone.Date
	RemainingDays float64
	Note          string
}

// SetVacationOpeningRepository wires the vacation opening repository (#2132).
// Setter injection like SetDeletionAudit so existing constructor call sites
// and unit fixtures stay unchanged; nil means summaries carry no takeover and
// the write paths fail loudly.
func (s *staffAbsenceService) SetVacationOpeningRepository(repo activeModels.StaffVacationOpeningRepository) {
	s.openingRepo = repo
}

// GetVacationOpening returns the takeover row for a staff/year, or nil.
func (s *staffAbsenceService) GetVacationOpening(ctx context.Context, staffID int64, year int) (*activeModels.StaffVacationOpening, error) {
	if staffID <= 0 {
		return nil, fmt.Errorf("%w: staff id is required", ErrVacationOpeningInvalid)
	}
	if s.openingRepo == nil {
		return nil, nil
	}
	return s.openingRepo.GetByStaffAndYear(ctx, staffID, year)
}

// SetVacationOpening books the vacation takeover for the Stichtag's year.
func (s *staffAbsenceService) SetVacationOpening(ctx context.Context, staffID, decidedBy int64, req SetVacationOpeningRequest) (*activeModels.StaffVacationOpening, error) {
	if s.openingRepo == nil {
		return nil, fmt.Errorf("vacation opening repository is not configured")
	}
	if staffID <= 0 {
		return nil, fmt.Errorf("%w: staff id is required", ErrVacationOpeningInvalid)
	}
	if decidedBy <= 0 {
		return nil, fmt.Errorf("%w: decided_by is required", ErrVacationOpeningInvalid)
	}
	if req.EffectiveDate.IsZero() {
		return nil, fmt.Errorf("%w: effective_date is required", ErrVacationOpeningInvalid)
	}
	if strings.TrimSpace(req.Note) == "" {
		return nil, fmt.Errorf("%w: note is required", ErrVacationOpeningInvalid)
	}
	// Same reasoning as the Stundenkonto opening: the Stichtag needs a closed
	// cutoff, and the bulk import applies ONE Stichtag to both sides.
	if !req.EffectiveDate.Before(timezone.TodayDate()) {
		return nil, fmt.Errorf("%w: effective_date must be before today", ErrVacationOpeningInvalid)
	}
	// Vacation absences use the same per-staff advisory lock. Holding it from
	// the pre-cutoff check through the insert makes the double-count guard
	// serializable with concurrent absence writes.
	if err := s.absenceRepo.LockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, fmt.Errorf("failed to lock staff absence writes: %w", err)
	}

	year := req.EffectiveDate.Year
	existing, err := s.openingRepo.GetByStaffAndYear(ctx, staffID, year)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing vacation opening: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("%w: booked %s", ErrVacationOpeningExists, existing.EffectiveDate.String())
	}
	if err := s.rejectVacationAbsencesBefore(ctx, staffID, req.EffectiveDate); err != nil {
		return nil, err
	}

	quota, err := s.quotaRepo.GetByStaffAndYear(ctx, staffID, year)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quota for vacation opening: %w", err)
	}
	entitled, carryover := defaultEntitledDays, 0.0
	if quota != nil {
		entitled, carryover = quota.EntitledDays, quota.CarryoverDays
	}
	takenBefore := entitled + carryover - req.RemainingDays

	now := time.Now()
	opening := &activeModels.StaffVacationOpening{
		StaffID:              staffID,
		Year:                 year,
		EffectiveDate:        req.EffectiveDate,
		TakenBeforeDays:      takenBefore,
		EnteredRemainingDays: req.RemainingDays,
		Note:                 req.Note,
		DecidedBy:            decidedBy,
		DecidedAt:            now,
	}
	opening.CreatedAt = now
	opening.UpdatedAt = now
	opening.SetTenantID(tenant.FromContext(ctx))
	if err := opening.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrVacationOpeningInvalid, err.Error())
	}
	if err := s.openingRepo.Create(ctx, opening); err != nil {
		if modelBase.IsUniqueViolationOn(err, vacationOpeningUniqueConstraintName) {
			return nil, fmt.Errorf("%w: %d", ErrVacationOpeningExists, year)
		}
		return nil, fmt.Errorf("failed to create vacation opening: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return opening, nil
}

// ValidateVacationOpeningAbsencesBefore exposes the read-only half of the
// opening guard for import previews. It deliberately takes no lock: imports
// validate rows in file order before their sorted creation pass, and retaining
// per-staff locks here could deadlock concurrent previews or imports. The
// actual SetVacationOpening path still locks through insertion.
func (s *staffAbsenceService) ValidateVacationOpeningAbsencesBefore(ctx context.Context, staffID int64, effectiveDate timezone.Date) error {
	return s.rejectVacationAbsencesBefore(ctx, staffID, effectiveDate)
}

// rejectVacationBeforeOpening keeps later vacation mutations from invalidating
// an already-booked takeover. Callers hold LockStaffAbsenceWrites, the same
// lock SetVacationOpening holds while checking and inserting its row.
func (s *staffAbsenceService) rejectVacationBeforeOpening(ctx context.Context, absence *activeModels.StaffAbsence) error {
	if absence.AbsenceType != activeModels.AbsenceTypeVacation || s.openingRepo == nil {
		return nil
	}
	for year := absence.DateStart.Year; year <= absence.DateEnd.Year; year++ {
		opening, err := s.openingRepo.GetByStaffAndYear(ctx, absence.StaffID, year)
		if err != nil {
			return fmt.Errorf("failed to check vacation opening: %w", err)
		}
		if opening == nil {
			continue
		}
		yearStart := timezone.NewDate(year, time.January, 1)
		if vacationAbsenceHasWorkingDayBefore(absence, yearStart, opening.EffectiveDate) {
			return fmt.Errorf("%w: vacation absence contains a working day before opening date %s", ErrVacationOpeningAbsencesBeforeCutoff, opening.EffectiveDate.String())
		}
	}
	return nil
}

// DeleteVacationOpening removes a takeover row and writes an append-only
// tombstone (audit.time_tracking_deletions) in the same tenant transaction.
func (s *staffAbsenceService) DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) error {
	if s.openingRepo == nil {
		return fmt.Errorf("vacation opening repository is not configured")
	}
	if staffID <= 0 || deletedBy <= 0 {
		return fmt.Errorf("%w: staff id and deleting staff id are required", ErrVacationOpeningInvalid)
	}
	opening, err := s.openingRepo.GetByStaffAndYear(ctx, staffID, year)
	if err != nil {
		return fmt.Errorf("failed to load vacation opening: %w", err)
	}
	if opening == nil {
		return fmt.Errorf("%w: staff %d year %d", ErrVacationOpeningNotFound, staffID, year)
	}
	if s.deletionRepo == nil {
		return fmt.Errorf("time tracking deletion audit repository is not configured")
	}
	payload, err := json.Marshal(opening)
	if err != nil {
		return fmt.Errorf("failed to snapshot vacation opening for deletion audit: %w", err)
	}
	if err := s.deletionRepo.Create(ctx, &auditModels.TimeTrackingDeletion{
		StaffID:   staffID,
		Source:    auditModels.TimeTrackingDeletionSourceVacationOpening,
		SourceID:  opening.ID,
		DeletedBy: deletedBy,
		Payload:   payload,
		Note:      opening.Note,
	}); err != nil {
		return fmt.Errorf("failed to write deletion audit: %w", err)
	}
	if err := s.openingRepo.Delete(ctx, opening.ID); err != nil {
		return fmt.Errorf("failed to delete vacation opening: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

// rejectVacationAbsencesBefore blocks a takeover when vacation absences that
// count against the quota already exist before the Stichtag in its year.
func (s *staffAbsenceService) rejectVacationAbsencesBefore(ctx context.Context, staffID int64, effectiveDate timezone.Date) error {
	yearStart := timezone.NewDate(effectiveDate.Year, time.January, 1)
	yearEnd := timezone.NewDate(effectiveDate.Year, time.December, 31)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
	if err != nil {
		return fmt.Errorf("failed to check absences for vacation opening: %w", err)
	}
	for _, a := range absences {
		if a.AbsenceType != activeModels.AbsenceTypeVacation {
			continue
		}
		switch a.Status {
		case activeModels.AbsenceStatusDeclined, activeModels.AbsenceStatusCanceled:
			continue
		}
		if vacationAbsenceHasWorkingDayBefore(a, yearStart, effectiveDate) {
			return fmt.Errorf(
				"%w: absence %s-%s contains a working day before %s",
				ErrVacationOpeningAbsencesBeforeCutoff,
				a.DateStart.String(), a.DateEnd.String(), effectiveDate.String(),
			)
		}
	}
	return nil
}

// vacationAbsenceHasWorkingDayBefore reports whether the absence spends any
// vacation quota before cutoff within the requested calendar year. Vacation
// quota currently counts Monday through Friday only; holidays are deliberately
// not excluded by countWorkingDays yet.
func vacationAbsenceHasWorkingDayBefore(absence *activeModels.StaffAbsence, yearStart, cutoff timezone.Date) bool {
	from := absence.DateStart
	if from.Before(yearStart) {
		from = yearStart
	}
	to := absence.DateEnd
	cutoffPreviousDay := cutoff.AddDays(-1)
	if cutoffPreviousDay.Before(to) {
		to = cutoffPreviousDay
	}
	if to.Before(from) {
		return false
	}
	startHalf, endHalf := effectiveBoundaryHalfDays(absence)
	if from != absence.DateStart {
		startHalf = false
	}
	if to != absence.DateEnd {
		endHalf = false
	}
	return countWorkingDays(from, to, startHalf, endHalf) > 0
}
