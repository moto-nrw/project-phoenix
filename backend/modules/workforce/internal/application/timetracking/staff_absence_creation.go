package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CreateAbsence creates a new absence record for the caller themself.
func (s *staffAbsenceService) CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	return s.CreateOwnAbsence(ctx, staffID, nil, req)
}

// CreateOwnAbsence is the self-service entry point. Comp-time absences change
// the Stundenkonto and therefore require the manager-authorized staff route.
func (s *staffAbsenceService) CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	if req.AbsenceType == AbsenceTypeCompTime {
		return nil, ErrManagerControlledAbsence
	}
	if req.AbsenceType == AbsenceTypeVacation {
		return nil, fmt.Errorf("vacation absences must be requested through the vacation flow")
	}
	if req.AbsenceTypeID != nil && s.absenceTypes != nil {
		absenceType, err := s.absenceTypes.GetAbsenceType(ctx, *req.AbsenceTypeID)
		if err != nil {
			return nil, err
		}
		if absenceType.AllowanceEnabled {
			return nil, ErrManagerControlledAbsence
		}
	}
	return s.CreateAbsenceFor(ctx, staffID, staffID, actorAccountID, req)
}

// CreateAbsenceFor creates an absence for subjectStaffID on createdByStaffID's
// behalf (#1843 admin sick reporting; self-service passes the same id twice).
// A full-day sick absence cascades into the plans; the merge path cascades
// over the MERGED range of the primary row, which the idempotency guards in
// the syncer make safe to re-run.
func (s *staffAbsenceService) CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	// Resolve the school-defined art first (#2403): it, not the client, decides
	// the canonical type every guard below and every later calculation reads.
	selectedType, typeID, baseType, err := s.resolveAbsenceTypeSelection(ctx, req.AbsenceTypeID, req.AbsenceType)
	if err != nil {
		return nil, err
	}
	req.AbsenceTypeID, req.AbsenceType = typeID, baseType

	// The Leitung books vacation directly, without a request (#3256). Staff
	// never reach this branch: CreateOwnAbsence sends them to RequestVacation.
	if req.AbsenceType == AbsenceTypeVacation {
		return s.createDirectVacation(ctx, subjectStaffID, createdByStaffID, req)
	}

	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if err = validateSickAbsenceRange(req.AbsenceType, dateStart, dateEnd); err != nil {
		return nil, err
	}
	if err := s.rejectPreAccountCompTime(ctx, req.AbsenceType, dateStart, dateEnd); err != nil {
		return nil, err
	}
	if err := s.lockStaffAbsenceWrites(ctx, subjectStaffID); err != nil {
		return nil, err
	}
	if req.AbsenceTypeID != nil {
		if _, err := s.absenceTypes.PreviewAllowanceBooking(
			ctx, subjectStaffID, *req.AbsenceTypeID, dateStart, dateEnd, req.HalfDay,
		); err != nil {
			return nil, err
		}
	}

	// Check for overlapping absences, merge if same type, reject if different type.
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, subjectStaffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}

	var resp *StaffAbsenceResponse
	if blocking := filterBlockingAbsences(existing); len(blocking) > 0 {
		if selectedType != nil && selectedType.AllowanceEnabled {
			return nil, ErrAllowanceBookingOverlap
		}
		resp, err = s.mergeOverlappingAbsences(ctx, blocking, dateStart, dateEnd, createdByStaffID, req)
	} else {
		if err := validateSingleDayHalfDayAbsence(req.AbsenceType, req.HalfDay, dateStart, dateEnd); err != nil {
			return nil, err
		}
		s.warnIfWorkSessionsExist(ctx, subjectStaffID, dateStart, dateEnd)
		resp, err = s.createNewAbsence(ctx, subjectStaffID, createdByStaffID, dateStart, dateEnd, req)
	}
	if err != nil {
		return nil, err
	}

	if err := s.cascadeSickReport(ctx, resp.StaffAbsence, createdByStaffID, actorAccountID); err != nil {
		return nil, err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return resp, nil
}

// cascadeSickReport fans a full-day sick report out into the plans (#1843).
// Fail-closed: an error aborts the surrounding create so a sick report whose
// plan effects half-applied never commits.
func (s *staffAbsenceService) cascadeSickReport(ctx context.Context, absence *StaffAbsence, actorStaffID int64, actorAccountID *int64) error {
	if absence.AbsenceType != AbsenceTypeSick || absence.HalfDay {
		return nil
	}
	if err := s.shiftPlanSyncer.MarkSickForRange(ctx, sickCascadeInput(absence, actorStaffID, actorAccountID)); err != nil {
		return fmt.Errorf("sick report saved nothing — plan cascade failed: %w", err)
	}
	return nil
}

// parseDateRange parses start and end date strings in ISO format.
func parseDateRange(startStr, endStr string) (calendar.Date, calendar.Date, error) {
	dateStart, err := calendar.ParseDate(startStr)
	if err != nil {
		return calendar.Date(""), calendar.Date(""), fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
	}
	dateEnd, err := calendar.ParseDate(endStr)
	if err != nil {
		return calendar.Date(""), calendar.Date(""), fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
	}
	// Reject reversed ranges here instead of letting them hit the DB check
	// constraint chk_sa_dates, which would surface as a 500 (#1420 review).
	// The "invalid" prefix is what the handlers classify as a 400.
	if dateEnd.Before(dateStart) {
		return calendar.Date(""), calendar.Date(""), fmt.Errorf("invalid date range: date_end must not be before date_start")
	}
	return dateStart, dateEnd, nil
}

// mergeOverlappingAbsences handles overlapping absences: rejects if different type, merges if same type.
func (s *staffAbsenceService) mergeOverlappingAbsences(
	ctx context.Context,
	existing []*StaffAbsence,
	dateStart, dateEnd calendar.Date,
	actorStaffID int64,
	req CreateAbsenceRequest,
) (*StaffAbsenceResponse, error) {
	// Check if all overlapping absences have the same type
	if err := validateSameAbsenceType(existing, req.AbsenceType); err != nil {
		return nil, err
	}
	if err := validateSameAbsenceTypeName(existing, req.AbsenceTypeID); err != nil {
		return nil, err
	}
	if err := validateSameMergeDuration(existing, req); err != nil {
		return nil, err
	}

	// Calculate merged date range
	mergedStart, mergedEnd := calculateMergedDateRange(existing, dateStart, dateEnd)
	if err := validateSickAbsenceRange(req.AbsenceType, mergedStart, mergedEnd); err != nil {
		return nil, err
	}
	if err := validateSingleDayHalfDayAbsence(req.AbsenceType, req.HalfDay, mergedStart, mergedEnd); err != nil {
		return nil, err
	}

	// Update the primary absence with merged range
	primary := existing[0]
	primary.DateStart = mergedStart
	primary.DateEnd = mergedEnd
	if req.Note != "" && primary.Note == "" {
		primary.Note = req.Note
	}
	primary.UpdatedAt = time.Now()

	if err := s.absenceRepo.Update(ctx, primary); err != nil {
		return nil, fmt.Errorf("failed to merge absence: %w", err)
	}

	// The secondaries are about to be deleted; their plan stamps must move to
	// the surviving primary or its eventual deletion would miss those rows
	// (#1843). Fail closed — a dangling stamp breaks the reversal invariant.
	if primary.AbsenceType == AbsenceTypeSick {
		for _, secondary := range existing[1:] {
			if err := s.shiftPlanSyncer.ReassignSickStamps(ctx, secondary.ID, primary.ID); err != nil {
				return nil, fmt.Errorf("failed to merge absence — plan stamp reassignment failed: %w", err)
			}
		}
	}

	// Delete remaining overlapping absences. The reassigned provenance and the
	// primary update must roll back if even one secondary cannot be removed.
	if err := s.deleteRemainingAbsences(ctx, existing[1:], actorStaffID); err != nil {
		return nil, err
	}

	return s.withLabel(ctx, toAbsenceResponse(primary)), nil
}

// validateSameMergeDuration rejects merging half-day with full-day rows for
// the duration-sensitive types: sick cascades per day, comp_time deducts the
// Stundenkonto. The merge keeps the primary's HalfDay flag, so a mismatch
// would silently rewrite the requested duration (#1420).
func validateSameMergeDuration(existing []*StaffAbsence, req CreateAbsenceRequest) error {
	if req.AbsenceType != AbsenceTypeSick && req.AbsenceType != AbsenceTypeCompTime {
		return nil
	}
	for _, absence := range existing {
		if absence.HalfDay != req.HalfDay {
			if req.AbsenceType == AbsenceTypeCompTime {
				return fmt.Errorf("invalid comp_time absence: half-day and full-day entries cannot be merged")
			}
			return fmt.Errorf("invalid sick absence: half-day and full-day reports cannot be merged")
		}
	}
	return nil
}

func validateSickAbsenceRange(absenceType string, start, end calendar.Date) error {
	if absenceType != AbsenceTypeSick || start.IsZero() || end.IsZero() || end.Before(start) {
		return nil
	}
	if start.DaysUntil(end)+1 > maxSickAbsenceRangeDays {
		return fmt.Errorf("invalid sick absence: date range exceeds %d days", maxSickAbsenceRangeDays)
	}
	return nil
}

func validateSingleDayHalfDayAbsence(absenceType string, halfDay bool, start, end calendar.Date) error {
	if !halfDay || start == end {
		return nil
	}
	switch absenceType {
	case AbsenceTypeSick:
		return fmt.Errorf("invalid sick absence: half-day reports must cover exactly one date")
	case AbsenceTypeCompTime:
		return fmt.Errorf("invalid comp_time absence: half-day reports must cover exactly one date")
	}
	return nil
}

// rejectPreAccountCompTime fails a comp_time absence dated before the account
// start: balance aggregation begins at the anchor, so such an absence would
// appear in the history without ever reducing the Stundenkonto (mirrors
// rejectPreAccountDate on balance adjustments, #1420).
func (s *staffAbsenceService) rejectPreAccountCompTime(ctx context.Context, absenceType string, dateStart, dateEnd calendar.Date) error {
	if absenceType != AbsenceTypeCompTime {
		return nil
	}
	anchor, err := resolveAccountAnchor(ctx, s.settings, s.getLogger(), monthOf(s.today()))
	if err != nil {
		return fmt.Errorf("failed to resolve account start for comp_time absence: %w", err)
	}
	if dateStart.Before(anchor) {
		return fmt.Errorf("invalid comp_time absence: date_start %s lies before the account start %s", dateStart.String(), anchor.String())
	}
	// Mirror the Monatskarte's future bound: the overdraft guard reads the
	// closing balance before dateStart, which has no defined result beyond
	// the carry-chain horizon (#1420 review).
	if horizon := monthOf(s.today()).addMonths(maxFutureMonths); horizon.before(monthOf(dateEnd)) {
		return fmt.Errorf("invalid comp_time absence: date_end %s is more than %d months ahead", dateEnd.String(), maxFutureMonths)
	}
	return nil
}

func (s *staffAbsenceService) lockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	if err := s.absenceRepo.LockStaffAbsenceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff absence writes: %w", err)
	}
	return nil
}

// validateSameAbsenceTypeName keeps the merge honest for school-defined
// Abwesenheitsarten (#2403). Two overlapping rows can share the canonical type
// "Sonstige" and still be a "Regenerationstag" and a "Ferienzeit" — merging
// them would silently relabel one of them, so an unequal art is an overlap
// conflict, exactly like an unequal type.
func validateSameAbsenceTypeName(existing []*StaffAbsence, absenceTypeID *int64) error {
	for _, e := range existing {
		if !sameAbsenceTypeID(e.AbsenceTypeID, absenceTypeID) {
			return fmt.Errorf("absence overlaps with an existing absence of a different Abwesenheitsart from %s to %s",
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

func sameAbsenceTypeID(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// validateSameAbsenceType checks that all existing absences match the requested type.
func validateSameAbsenceType(existing []*StaffAbsence, absenceType string) error {
	for _, e := range existing {
		if e.AbsenceType != absenceType {
			return fmt.Errorf("absence overlaps with existing %s absence from %s to %s",
				e.AbsenceType,
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

// calculateMergedDateRange expands the date range to cover all overlapping absences.
func calculateMergedDateRange(existing []*StaffAbsence, dateStart, dateEnd calendar.Date) (calendar.Date, calendar.Date) {
	mergedStart := dateStart
	mergedEnd := dateEnd
	for _, e := range existing {
		if e.DateStart.Before(mergedStart) {
			mergedStart = e.DateStart
		}
		if e.DateEnd.After(mergedEnd) {
			mergedEnd = e.DateEnd
		}
	}
	return mergedStart, mergedEnd
}

// deleteRemainingAbsences deletes absences that were merged into the primary.
func (s *staffAbsenceService) deleteRemainingAbsences(ctx context.Context, absences []*StaffAbsence, actorStaffID int64) error {
	for _, e := range absences {
		if err := s.writeAbsenceDeletionAudit(ctx, e, actorStaffID); err != nil {
			return fmt.Errorf("failed to audit merged absence %d: %w", e.ID, err)
		}
		if err := s.absenceRepo.Delete(ctx, e.ID); err != nil {
			return fmt.Errorf("failed to delete merged absence %d: %w", e.ID, err)
		}
	}
	return nil
}

// warnIfWorkSessionsExist logs a warning if work sessions exist in the date range.
func (s *staffAbsenceService) warnIfWorkSessionsExist(ctx context.Context, staffID int64, dateStart, dateEnd calendar.Date) {
	sessions, err := s.workSessionRepo.GetHistoryByStaffID(ctx, staffID, dateStart, dateEnd)
	if err == nil && len(sessions) > 0 {
		slog.Default().WarnContext(ctx, "work sessions exist in absence range",
			slog.Int("session_count", len(sessions)),
			slog.Int64("staff_id", staffID),
			slog.String("date_start", dateStart.String()),
			slog.String("date_end", dateEnd.String()))
	}
}

// createNewAbsence creates a new absence record in the database. createdBy
// may differ from staffID when an admin files the report (#1843).
func (s *staffAbsenceService) createNewAbsence(
	ctx context.Context,
	staffID int64,
	createdBy int64,
	dateStart, dateEnd calendar.Date,
	req CreateAbsenceRequest,
) (*StaffAbsenceResponse, error) {
	now := time.Now()
	absence := &StaffAbsence{
		StaffID:       staffID,
		AbsenceType:   req.AbsenceType,
		AbsenceTypeID: req.AbsenceTypeID,
		DateStart:     dateStart,
		DateEnd:       dateEnd,
		HalfDay:       req.HalfDay,
		Note:          req.Note,
		Status:        AbsenceStatusReported,
		CreatedBy:     createdBy,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now

	absence.SetTenantID(tenant.FromContext(ctx))
	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create absence: %w", err)
	}

	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}
