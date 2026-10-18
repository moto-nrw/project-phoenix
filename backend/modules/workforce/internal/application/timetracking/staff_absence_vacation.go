package timetracking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func effectiveBoundaryHalfDays(a *StaffAbsence) (bool, bool) {
	if a.HalfDay && !a.StartHalfDay && !a.EndHalfDay {
		return true, true
	}
	return a.StartHalfDay, a.EndHalfDay
}

func isWorkingDay(d calendar.Date) bool {
	w := d.Weekday()
	return w != time.Saturday && w != time.Sunday
}

// Vacation workflow (Tranche 4)

// countWorkingDays returns Mon-Fri count between two dates inclusive,
// minus 0.5 for each boundary half-day. Personio-style: first day half
// and last day half are independent flags. Feiertage are NOT excluded
// yet. That lives in Tranche 3 (Issue #1231).
func countWorkingDays(from, to calendar.Date, startHalf, endHalf bool) float64 {
	if to.Before(from) {
		return 0
	}
	count := 0
	for d := from; !d.After(to); d = d.AddDays(1) {
		if isWorkingDay(d) {
			count++
		}
	}
	result := float64(count)
	// Single-day range: only one boundary exists, so a single half flag halves it.
	if from == to {
		if isWorkingDay(from) && (startHalf || endHalf) {
			result -= 0.5
		}
		return result
	}
	if startHalf && isWorkingDay(from) {
		result -= 0.5
	}
	if endHalf && isWorkingDay(to) {
		result -= 0.5
	}
	return result
}

func (s *staffAbsenceService) RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error) {
	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if isBeforeLocalToday(dateStart, time.Now()) {
		return nil, fmt.Errorf("vacation request must start today or in the future")
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}

	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	// Declined and canceled rows do not block the time slot, the staff
	// member is free to request the same dates again. Only requested,
	// approved and reported absences hold the range.
	for _, e := range existing {
		if blocksAbsenceRange(e.Status) {
			return nil, fmt.Errorf("dates overlap with an existing absence")
		}
	}

	workingDays := countWorkingDays(dateStart, dateEnd, req.StartHalfDay, req.EndHalfDay)
	if workingDays <= 0 {
		return nil, fmt.Errorf("vacation range contains no working days")
	}

	now := time.Now()
	// HalfDay legacy flag stays in sync: true iff either boundary is half.
	halfDay := req.StartHalfDay || req.EndHalfDay
	absence := &StaffAbsence{
		StaffID:           staffID,
		AbsenceType:       AbsenceTypeVacation,
		DateStart:         dateStart,
		DateEnd:           dateEnd,
		HalfDay:           halfDay,
		StartHalfDay:      req.StartHalfDay,
		EndHalfDay:        req.EndHalfDay,
		Note:              req.Note,
		Status:            AbsenceStatusRequested,
		CreatedBy:         staffID,
		WorkingDays:       &workingDays,
		RequestedAt:       now,
		SubstituteStaffID: req.SubstituteStaffID,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now
	absence.SetTenantID(tenant.FromContext(ctx))

	// Open requests already hold their days, so a new one must fit next to them.
	if err := s.ensureVacationFits(ctx, absence, 0, true); err != nil {
		return nil, err
	}
	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create vacation request: %w", err)
	}
	if err := s.notifyAbsenceRequested(ctx, absence); err != nil {
		return nil, fmt.Errorf("enqueue absence mail: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error) {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	absence, err = s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != AbsenceStatusRequested &&
		absence.Status != AbsenceStatusQuestion {
		return nil, fmt.Errorf("only requested absences can be approved")
	}
	if err := s.rejectVacationBeforeOpening(ctx, absence); err != nil {
		return nil, err
	}
	// Other open requests are judged at their own approval; only what is
	// already spent must leave room for this one.
	if absence.AbsenceType == AbsenceTypeVacation {
		if err := s.ensureVacationFits(ctx, absence, absence.ID, false); err != nil {
			return nil, err
		}
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = AbsenceStatusApproved
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = note
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to approve absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence approval: %w", err)
	}
	if err := s.notifyAbsenceDecision(ctx, absence); err != nil {
		return nil, fmt.Errorf("enqueue absence mail: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) DenyAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, reason string) (*StaffAbsenceResponse, error) {
	if reason == "" {
		return nil, fmt.Errorf("decline reason is required")
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	absence, err = s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != AbsenceStatusRequested &&
		absence.Status != AbsenceStatusQuestion {
		return nil, fmt.Errorf("only requested absences can be declined")
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = AbsenceStatusDeclined
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = reason
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to decline absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, reason); err != nil {
		return nil, fmt.Errorf("failed to audit absence decline: %w", err)
	}
	if err := s.notifyAbsenceDecision(ctx, absence); err != nil {
		return nil, fmt.Errorf("enqueue absence mail: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) QuestionAbsence(ctx context.Context, absenceID int64, actorAccountID int64, note string) (*StaffAbsenceResponse, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, fmt.Errorf("question note is required")
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	absence, err = s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != AbsenceStatusRequested {
		return nil, fmt.Errorf("only requested absences can be questioned")
	}
	fromStatus := absence.Status
	absence.Status = AbsenceStatusQuestion
	// The Leitung's question lives in decision_note; ApprovedBy/At stay nil
	// because no decision has been made yet. History is in the audit rows.
	absence.DecisionNote = note
	absence.UpdatedAt = time.Now()

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to question absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence question: %w", err)
	}
	if err := s.notifyAbsenceDecision(ctx, absence); err != nil {
		return nil, fmt.Errorf("enqueue absence mail: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) ResubmitAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64, note string) (*StaffAbsenceResponse, error) {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil, fmt.Errorf("resubmit note is required")
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.StaffID != staffID {
		return nil, fmt.Errorf("can only resubmit own absences")
	}
	if absence.Status != AbsenceStatusQuestion {
		return nil, fmt.Errorf("only absences with a question can be resubmitted")
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = AbsenceStatusRequested
	absence.Note = note
	// Re-stamp so the request re-enters the inbox at its resubmit time.
	absence.RequestedAt = now
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to resubmit absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence resubmit: %w", err)
	}
	// A resubmit re-enters the inbox, so approvers get the "received" mail again.
	if err := s.notifyAbsenceRequested(ctx, absence); err != nil {
		return nil, fmt.Errorf("enqueue absence mail: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) CancelAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64) error {
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return fmt.Errorf("absence not found")
	}
	if absence.StaffID != staffID {
		return fmt.Errorf("can only cancel own absences")
	}
	// MA can cancel requested/questioned or approved future vacation. Past
	// approved absences become historical record and are not cancelable from
	// the UI.
	if absence.Status != AbsenceStatusRequested &&
		absence.Status != AbsenceStatusQuestion &&
		absence.Status != AbsenceStatusApproved {
		return fmt.Errorf("only pending or approved absences can be canceled")
	}
	if absence.Status == AbsenceStatusApproved &&
		isBeforeLocalToday(absence.DateStart, time.Now()) {
		return fmt.Errorf("past absences cannot be canceled")
	}
	fromStatus := absence.Status
	absence.Status = AbsenceStatusCanceled
	absence.UpdatedAt = time.Now()
	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return fmt.Errorf("failed to cancel absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, ""); err != nil {
		return fmt.Errorf("failed to audit absence cancellation: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffAbsenceService) createAudit(ctx context.Context, absenceID int64, actorAccountID int64, fromStatus string, toStatus string, note string) error {
	if s.auditRepo == nil {
		return fmt.Errorf("staff absence audit repository is not configured")
	}
	audit := &StaffAbsenceAudit{
		AbsenceID:  absenceID,
		FromStatus: &fromStatus,
		ToStatus:   toStatus,
		ActorID:    actorAccountID,
		Note:       note,
	}
	return s.auditRepo.Create(ctx, audit)
}

func isBeforeLocalToday(date calendar.Date, now time.Time) bool {
	return date.Before(calendar.DateFromTime(now))
}
