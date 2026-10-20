package timetracking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *staffAbsenceService) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsenceResponse, error) {
	absence, err := s.loadSelfEditableAbsence(ctx, staffID, absenceID, req)
	if err != nil {
		return nil, err
	}
	before := *absence
	if err := validateAbsenceUpdate(absence, req); err != nil {
		return nil, err
	}

	// Apply updates from request
	if err := applyAbsenceUpdates(absence, req); err != nil {
		return nil, err
	}
	if err := s.repinAbsenceType(ctx, absence, &before, req); err != nil {
		return nil, err
	}
	if err := s.validateUpdatedAbsenceRange(ctx, absence); err != nil {
		return nil, err
	}

	// Check for overlapping absences (excluding self)
	if err := s.checkOverlapExcludingSelf(ctx, staffID, absenceID, absence.DateStart, absence.DateEnd); err != nil {
		return nil, err
	}

	absence.UpdatedAt = time.Now()

	if err := absence.Validate(); err != nil {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid absence data: %w", err)}
	}

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to update absence: %w", err)
	}

	if err := s.reconcileUpdatedSickAbsence(ctx, &before, absence, staffID, actorAccountID); err != nil {
		return nil, err
	}

	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

// loadSelfEditableAbsence takes the staff member's absence lock and returns
// the absence when it is theirs and not manager-controlled.
func (s *staffAbsenceService) loadSelfEditableAbsence(ctx context.Context, staffID, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsence, error) {
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, workforce.ErrAbsenceNotFound
	}

	// Verify ownership
	if absence.StaffID != staffID {
		return nil, workforce.ErrAbsenceUpdateNotOwned
	}
	if err := s.rejectManagerControlledCustomUpdate(ctx, absence, req); err != nil {
		return nil, err
	}
	if absence.AbsenceType == AbsenceTypeCompTime ||
		(req.AbsenceType != nil && *req.AbsenceType == AbsenceTypeCompTime) ||
		isDirectVacation(absence) {
		return nil, ErrManagerControlledAbsence
	}
	return absence, nil
}

// repinAbsenceType keeps the Abwesenheitsart and the canonical type of an
// edited absence consistent. A re-pointed Abwesenheitsart (#2403) also
// re-pins the canonical type, so the name and the arithmetic can never drift
// apart on an edit either. An unchanged retired art is deliberately
// preserved: historic entries remain editable, while resolving a newly
// selected retired art still rejects it.
func (s *staffAbsenceService) repinAbsenceType(ctx context.Context, absence, before *StaffAbsence, req UpdateAbsenceRequest) error {
	if req.AbsenceTypeIDSet || req.AbsenceTypeID != nil {
		return s.repinSelectedAbsenceType(ctx, absence, before, req.AbsenceTypeID)
	}
	if req.AbsenceType != nil && before.AbsenceTypeID != nil {
		// A canonical type update without an art ID means the standard type. Do
		// not retain the old custom ID, as that pair violates the DB constraint.
		absence.AbsenceTypeID = nil
	}
	return nil
}

// repinSelectedAbsenceType applies an Abwesenheitsart the edit names
// explicitly, possibly as null.
func (s *staffAbsenceService) repinSelectedAbsenceType(ctx context.Context, absence, before *StaffAbsence, typeID *int64) error {
	if before.AbsenceType == AbsenceTypeSick &&
		!sameAbsenceTypeID(typeID, before.AbsenceTypeID) {
		return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid absence type change: sick absences must be deleted and re-created, not converted")}
	}
	if !sameAbsenceTypeID(typeID, before.AbsenceTypeID) {
		_, resolvedID, baseType, err := s.resolveAbsenceTypeSelection(ctx, typeID, absence.AbsenceType)
		if err != nil {
			return err
		}
		absence.AbsenceTypeID, absence.AbsenceType = resolvedID, baseType
		return nil
	}
	absence.AbsenceTypeID = before.AbsenceTypeID
	// Only a kept custom art re-pins the canonical type. With no art on
	// either side there is nothing to pin to, and restoring the old type
	// here would swallow a plain standard-type change: the client sends
	// absence_type_id: null alongside every standard selection, so this
	// branch is exactly the "Fortbildung → Sonstige" case.
	if before.AbsenceTypeID != nil {
		absence.AbsenceType = before.AbsenceType
	}
	return nil
}

// validateUpdatedAbsenceRange re-checks the range rules on the edited absence.
func (s *staffAbsenceService) validateUpdatedAbsenceRange(ctx context.Context, absence *StaffAbsence) error {
	if err := validateSickAbsenceRange(absence.AbsenceType, absence.DateStart, absence.DateEnd); err != nil {
		return err
	}
	if err := validateSingleDayHalfDayAbsence(absence.AbsenceType, absence.HalfDay, absence.DateStart, absence.DateEnd); err != nil {
		return err
	}
	return s.rejectVacationBeforeOpening(ctx, absence)
}

func (s *staffAbsenceService) rejectManagerControlledCustomUpdate(ctx context.Context, current *StaffAbsence, req UpdateAbsenceRequest) error {
	if s.absenceTypes == nil {
		return nil
	}
	ids := []*int64{current.AbsenceTypeID}
	if req.AbsenceTypeIDSet || req.AbsenceTypeID != nil {
		ids = append(ids, req.AbsenceTypeID)
	}
	for _, id := range ids {
		if err := s.rejectManagerControlledCustomType(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *staffAbsenceService) rejectManagerControlledCustomType(ctx context.Context, id *int64) error {
	if id == nil || s.absenceTypes == nil {
		return nil
	}
	absenceType, err := s.absenceTypes.GetAbsenceType(ctx, *id)
	if err != nil {
		return err
	}
	if absenceType.AllowanceEnabled {
		return ErrManagerControlledAbsence
	}
	return nil
}

func validateAbsenceUpdate(absence *StaffAbsence, req UpdateAbsenceRequest) error {
	if isVacationWorkflowAbsence(absence) {
		return &workforce.TimeTrackingError{Kind: workforce.ErrVacationFlowRequired, Cause: fmt.Errorf("vacation workflow absences must be changed through the vacation flow")}
	}
	if req.AbsenceType != nil && *req.AbsenceType == AbsenceTypeVacation {
		return &workforce.TimeTrackingError{Kind: workforce.ErrVacationFlowRequired, Cause: fmt.Errorf("vacation absences must be requested through the vacation flow")}
	}
	// A type change into or out of "sick" would desync the #1843 plan cascade:
	// sick→other keeps the cancellations/marks but skips the reversal on
	// delete forever; other→sick pretends a cascade that never ran. Delete
	// and re-create instead. Date edits remain supported and reconcile their
	// plan-day difference below.
	if req.AbsenceType != nil && *req.AbsenceType != absence.AbsenceType &&
		(absence.AbsenceType == AbsenceTypeSick || *req.AbsenceType == AbsenceTypeSick) {
		return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid absence type change: sick absences must be deleted and re-created, not converted")}
	}
	// Same reasoning for the half-day flag: flipping it would desync the
	// already-applied (or deliberately skipped) cascade from the record.
	if absence.AbsenceType == AbsenceTypeSick &&
		req.HalfDay != nil && *req.HalfDay != absence.HalfDay {
		return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid absence change: sick absences cannot switch between half and full days — delete and re-create the report")}
	}
	return nil
}

func (s *staffAbsenceService) reconcileUpdatedSickAbsence(ctx context.Context, before, after *StaffAbsence, actorStaffID int64, actorAccountID *int64) error {
	if before.AbsenceType == AbsenceTypeSick && !before.HalfDay &&
		(before.DateStart != after.DateStart || before.DateEnd != after.DateEnd) {
		if err := s.shiftPlanSyncer.ReconcileSickRange(
			ctx,
			sickCascadeInput(before, actorStaffID, actorAccountID),
			sickCascadeInput(after, actorStaffID, actorAccountID),
		); err != nil {
			return fmt.Errorf("absence update saved nothing — plan cascade reconciliation failed: %w", err)
		}
	}
	return nil
}

// sickCascadeInput is the cascade's view of one sick report. The public
// contract carries the range as DateLayout strings.
func sickCascadeInput(absence *StaffAbsence, actorStaffID int64, actorAccountID *int64) SickCascadeInput {
	return SickCascadeInput{
		SubjectStaffID: absence.StaffID,
		DateStart:      absence.DateStart.String(),
		DateEnd:        absence.DateEnd.String(),
		SkipStartDay:   absence.StartHalfDay,
		SkipEndDay:     absence.EndHalfDay,
		AbsenceID:      absence.ID,
		ActorStaffID:   actorStaffID,
		ActorAccountID: actorAccountID,
	}
}

// applyAbsenceUpdates applies partial updates from the request to the absence.
func applyAbsenceUpdates(absence *StaffAbsence, req UpdateAbsenceRequest) error {
	if req.AbsenceType != nil {
		absence.AbsenceType = *req.AbsenceType
	}
	if req.DateStart != nil {
		dateStart, err := calendar.ParseDate(*req.DateStart)
		if err != nil {
			return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")}
		}
		absence.DateStart = dateStart
	}
	if req.DateEnd != nil {
		dateEnd, err := calendar.ParseDate(*req.DateEnd)
		if err != nil {
			return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")}
		}
		absence.DateEnd = dateEnd
	}
	if req.HalfDay != nil {
		absence.HalfDay = *req.HalfDay
	}
	if req.Note != nil {
		absence.Note = *req.Note
	}
	return nil
}

// checkOverlapExcludingSelf checks for overlapping absences, excluding the given absence ID.
func (s *staffAbsenceService) checkOverlapExcludingSelf(ctx context.Context, staffID, absenceID int64, dateStart, dateEnd calendar.Date) error {
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return fmt.Errorf("failed to check existing absences: %w", err)
	}
	for _, e := range existing {
		if e.ID != absenceID && blocksAbsenceRange(e.Status) {
			return &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceUpdatedDatesOverlap, Cause: fmt.Errorf("updated dates overlap with existing absence from %s to %s",
				e.DateStart,
				e.DateEnd)}
		}
	}
	return nil
}

func (s *staffAbsenceService) DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error {
	return s.DeleteOwnAbsence(ctx, staffID, nil, absenceID)
}

func (s *staffAbsenceService) DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error {
	return s.deleteAbsenceFor(ctx, staffID, staffID, actorAccountID, absenceID, false)
}

func (s *staffAbsenceService) DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error {
	return s.deleteAbsenceFor(ctx, subjectStaffID, actorStaffID, actorAccountID, absenceID, true)
}

func (s *staffAbsenceService) deleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64, allowManagerControlled bool) error {
	if err := s.lockStaffAbsenceWrites(ctx, subjectStaffID); err != nil {
		return err
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return workforce.ErrAbsenceNotFound
	}

	// Verify ownership
	if absence.StaffID != subjectStaffID {
		return workforce.ErrAbsenceDeleteNotOwned
	}
	if (absence.AbsenceType == AbsenceTypeCompTime || isDirectVacation(absence)) && !allowManagerControlled {
		return ErrManagerControlledAbsence
	}
	if !allowManagerControlled {
		if err := s.rejectManagerControlledCustomType(ctx, absence.AbsenceTypeID); err != nil {
			return err
		}
	}
	if isVacationWorkflowAbsence(absence) {
		return &workforce.TimeTrackingError{Kind: workforce.ErrVacationFlowRequired, Cause: fmt.Errorf("vacation workflow absences must be canceled through the vacation flow")}
	}

	// The reversal runs UNCONDITIONALLY: it is keyed purely by provenance
	// stamps (sick_absence_id == this id), so an absence that never cascaded
	// is a cheap no-op, while gating on type/half_day would let a mutated row
	// (e.g. half_day flipped after the cascade ran) orphan its stamps forever.
	if err := s.shiftPlanSyncer.ClearSickForRange(ctx, sickCascadeInput(absence, actorStaffID, actorAccountID)); err != nil {
		return fmt.Errorf("absence not deleted — plan cascade reversal failed: %w", err)
	}

	// Tombstone first, delete second, same tenant tx: the absence's own
	// audit rows CASCADE away with it, so this row is the only surviving
	// trace (#1417).
	if err := s.writeAbsenceDeletionAudit(ctx, absence, actorStaffID); err != nil {
		return err
	}
	if err := s.absenceRepo.Delete(ctx, absenceID); err != nil {
		return fmt.Errorf("failed to delete absence: %w", err)
	}

	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffAbsenceService) writeAbsenceDeletionAudit(ctx context.Context, absence *StaffAbsence, actorStaffID int64) error {
	if s.deletionRepo == nil {
		return fmt.Errorf("time tracking deletion audit repository is not configured")
	}
	StampAbsenceTypeLabels(ctx, s.absenceTypes, []*StaffAbsence{absence}, s.getLogger())
	payload, err := json.Marshal(absence)
	if err != nil {
		return fmt.Errorf("failed to snapshot absence for deletion audit: %w", err)
	}
	if err := s.deletionRepo.Create(ctx, &TimeTrackingDeletionEvent{
		StaffID:   absence.StaffID,
		Source:    "absence",
		SourceID:  absence.ID,
		DeletedBy: actorStaffID,
		Payload:   payload,
		Note:      absence.Note,
	}); err != nil {
		return fmt.Errorf("failed to write deletion audit: %w", err)
	}
	return nil
}
