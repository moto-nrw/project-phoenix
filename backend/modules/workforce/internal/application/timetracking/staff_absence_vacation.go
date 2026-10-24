package timetracking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

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
		return nil, workforce.ErrVacationRequestInPast
	}
	if err := s.lockVacationRequestRange(ctx, staffID, dateStart, dateEnd); err != nil {
		return nil, err
	}

	workingDays := countWorkingDays(dateStart, dateEnd, req.StartHalfDay, req.EndHalfDay)
	if workingDays <= 0 {
		return nil, workforce.ErrAbsenceNoWorkingDays
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
	absenceCommand := StaffAbsenceToCapability(absence)
	createdAbsence, err := s.absenceRepo.CreateStaffAbsence(ctx, &absenceCommand)
	err = absenceWriteError("create", err)
	if err == nil {
		applyAbsenceToRecord(absence, createdAbsence)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create vacation request: %w", err)
	}
	if err := s.notifyAbsenceRequested(ctx, absence); err != nil {
		return nil, fmt.Errorf("enqueue absence mail: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

func (s *staffAbsenceService) ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error) {
	storedAbsence, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence := absenceToRecord(storedAbsence)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	storedAbsence2, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence = absenceToRecord(storedAbsence2)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if absence.Status != AbsenceStatusRequested &&
		absence.Status != AbsenceStatusQuestion {
		return nil, workforce.ErrAbsenceNotApprovable
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

	updatedAbsence, err := s.absenceRepo.UpdateStaffAbsence(ctx, StaffAbsenceToCapability(absence))
	err = absenceUpdateError(err)
	if err == nil {
		applyAbsenceToRecord(absence, updatedAbsence)
	}
	if err != nil {
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
		return nil, workforce.ErrAbsenceDeclineNoteRequired
	}
	storedAbsence, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence := absenceToRecord(storedAbsence)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	storedAbsence2, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence = absenceToRecord(storedAbsence2)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if absence.Status != AbsenceStatusRequested &&
		absence.Status != AbsenceStatusQuestion {
		return nil, workforce.ErrAbsenceNotDeclinable
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = AbsenceStatusDeclined
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = reason
	absence.UpdatedAt = now

	updatedAbsence, err := s.absenceRepo.UpdateStaffAbsence(ctx, StaffAbsenceToCapability(absence))
	err = absenceUpdateError(err)
	if err == nil {
		applyAbsenceToRecord(absence, updatedAbsence)
	}
	if err != nil {
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
		return nil, workforce.ErrAbsenceQuestionNoteRequired
	}
	storedAbsence, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence := absenceToRecord(storedAbsence)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if err := s.lockStaffAbsenceWrites(ctx, absence.StaffID); err != nil {
		return nil, err
	}
	storedAbsence2, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence = absenceToRecord(storedAbsence2)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if absence.Status != AbsenceStatusRequested {
		return nil, workforce.ErrAbsenceNotQuestionable
	}
	fromStatus := absence.Status
	absence.Status = AbsenceStatusQuestion
	// The Leitung's question lives in decision_note; ApprovedBy/At stay nil
	// because no decision has been made yet. History is in the audit rows.
	absence.DecisionNote = note
	absence.UpdatedAt = time.Now()

	updatedAbsence, err := s.absenceRepo.UpdateStaffAbsence(ctx, StaffAbsenceToCapability(absence))
	err = absenceUpdateError(err)
	if err == nil {
		applyAbsenceToRecord(absence, updatedAbsence)
	}
	if err != nil {
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
		return nil, workforce.ErrAbsenceResubmitNoteRequired
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	storedAbsence, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence := absenceToRecord(storedAbsence)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}
	if absence.StaffID != staffID {
		return nil, workforce.ErrAbsenceResubmitNotOwned
	}
	if absence.Status != AbsenceStatusQuestion {
		return nil, workforce.ErrAbsenceNotQuestioned
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = AbsenceStatusRequested
	absence.Note = note
	// Re-stamp so the request re-enters the inbox at its resubmit time.
	absence.RequestedAt = now
	absence.UpdatedAt = now

	updatedAbsence, err := s.absenceRepo.UpdateStaffAbsence(ctx, StaffAbsenceToCapability(absence))
	err = absenceUpdateError(err)
	if err == nil {
		applyAbsenceToRecord(absence, updatedAbsence)
	}
	if err != nil {
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
	storedAbsence, err := s.absenceRepo.FindStaffAbsence(ctx, absenceID)
	absence := absenceToRecord(storedAbsence)
	err = rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	if err != nil {
		return workforce.ErrAbsenceNotFound
	}
	if absence.StaffID != staffID {
		return workforce.ErrAbsenceCancelNotOwned
	}
	// MA can cancel requested/questioned or approved future vacation. Past
	// approved absences become historical record and are not cancelable from
	// the UI.
	if absence.Status != AbsenceStatusRequested &&
		absence.Status != AbsenceStatusQuestion &&
		absence.Status != AbsenceStatusApproved {
		return workforce.ErrAbsenceNotCancelable
	}
	if absence.Status == AbsenceStatusApproved &&
		isBeforeLocalToday(absence.DateStart, time.Now()) {
		return workforce.ErrPastAbsenceCancel
	}
	fromStatus := absence.Status
	absence.Status = AbsenceStatusCanceled
	absence.UpdatedAt = time.Now()
	updatedAbsence, err := s.absenceRepo.UpdateStaffAbsence(ctx, StaffAbsenceToCapability(absence))
	err = absenceUpdateError(err)
	if err == nil {
		applyAbsenceToRecord(absence, updatedAbsence)
	}
	if err != nil {
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
	if err := audit.Validate(); err != nil {
		return err
	}
	value, err := s.auditRepo.RecordStaffAbsenceAudit(ctx, absenceAuditValue(audit))
	if err != nil {
		return absenceWriteError("create staff absence audit", err)
	}
	applyAbsenceAudit(audit, value)
	return nil
}

func isBeforeLocalToday(date calendar.Date, now time.Time) bool {
	return date.Before(calendar.DateFromTime(now))
}

func (s *staffAbsenceService) lockVacationRequestRange(ctx context.Context, staffID int64, dateStart, dateEnd calendar.Date) error {
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return err
	}

	absenceValues, err := s.absenceRepo.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{StaffID: staffID, OverlapFrom: dateStart.String(), OverlapTo: dateEnd.String(), Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}}})
	existing := absencesToRecord(absenceValues)
	err = rowReadError("get absences by staff and date range", err, nil)
	if err != nil {
		return fmt.Errorf("failed to check existing absences: %w", err)
	}
	// Declined and canceled rows do not block the time slot, the staff
	// member is free to request the same dates again. Only requested,
	// approved and reported absences hold the range.
	for _, e := range existing {
		if blocksAbsenceRange(e.Status) {
			return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceDatesOverlap, Cause: fmt.Errorf("dates overlap with an existing absence")}
		}
	}

	return nil
}
