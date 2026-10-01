package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// approvalContext is what an approval applies: the phase and request of the
// child, the requested selection, and the confirmed effective date.
type approvalContext struct {
	requestID     int64
	phase         *ports.BookingPhase
	selections    []careplan.OfferingChangeSelection
	effectiveFrom calendar.Date
}

// applyApproved re-validates the request against today's catalog and
// capacity and then performs the dated switch. The request may be weeks old;
// an offering can have been deactivated or filled up since. A failure leaves
// the row pending, so the office sees an actionable error instead of a
// request marked done that never applied. On success row carries the date
// the switch applied on.
func (s *OfferingChanges) applyApproved(
	ctx context.Context,
	row *careplan.OfferingChangeRequest,
	input careplan.OfferingChangeDecisionInput,
) (*careplan.OfferingAdjustmentResult, error) {
	approval, err := s.approvalContext(ctx, *row, input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	excluded := offeringIDSet(input.ExcludedAutoOfferingIDs)
	completeWithdrawal, err := s.checkApproval(ctx, *row, approval, excluded, input.CompleteWithdrawalConfirmed)
	if err != nil {
		return nil, err
	}
	// Keep the actual date in memory for the adjustment audit. Persist it
	// only after the switch succeeds, so a client error leaves the pending
	// row intact.
	requestedFrom := offeringChangeEffectiveFrom(*row)
	row.EffectiveFrom = approval.effectiveFrom.String()
	effectiveFrom := approval.effectiveFrom
	applied, err := s.deps.Bookings.AdjustOfferings(ctx, careplan.OfferingAdjustment{
		RequestID: approval.requestID, ChildID: row.RequestChildID, Offerings: adjustmentChoicesOf(approval.selections),
		ExcludedAutoAddTargetIDs: excluded, Reason: offeringChangeAdjustmentReason(*row, requestedFrom, input.Reason),
		ActorAccountID: input.ReviewedBy, ActorRole: cmpOr(strings.TrimSpace(input.ActorRole), "admin"),
		EffectiveFrom: &effectiveFrom,
		// Staff confirms the fully materialized result at review time; it may
		// differ from the state materialized when the request was filed.
		CompleteWithdrawalConfirmed: completeWithdrawal && input.CompleteWithdrawalConfirmed,
		Source:                      careplan.OfferingAdjustmentSourceRequest,
	})
	if err != nil {
		return nil, fmt.Errorf("offering change: apply: %w", err)
	}
	if err := s.deps.Rows.UpdateEffectiveFrom(ctx, row.ID, effectiveFrom); err != nil {
		return nil, fmt.Errorf("offering change: update applied effective date: %w", err)
	}
	return applied, nil
}

// approvalContext resolves the child's phase and the date an approval applies
// on, and refuses a date the care period or the child's care no longer
// reaches.
func (s *OfferingChanges) approvalContext(ctx context.Context, row careplan.OfferingChangeRequest, confirmed *calendar.Date) (*approvalContext, error) {
	if err := s.requireCareOfferings(ctx); err != nil {
		return nil, err
	}
	child, err := s.deps.Enrollment.Child(ctx, row.RequestChildID)
	if err != nil || child == nil {
		return nil, fmt.Errorf("offering change: load request child: %w", err)
	}
	request, err := s.deps.Enrollment.Request(ctx, child.RequestID)
	if err != nil || request == nil {
		return nil, fmt.Errorf("offering change: load request: %w", err)
	}
	phase, err := s.deps.Enrollment.Phase(ctx, request.PhaseID)
	if err != nil || phase == nil {
		return nil, fmt.Errorf("offering change: load phase: %w", err)
	}
	selections, err := requestSelections(row)
	if err != nil {
		return nil, err
	}
	// A request whose date passed while it waited applies as soon as
	// possible; a date the office confirmed itself is never moved.
	effectiveFrom, err := confirmedEffectiveFrom(confirmed, offeringChangeEffectiveFrom(row), s.todayDate(), phase)
	if err != nil {
		return nil, err
	}
	if effectiveFrom.After(phase.ServiceEnd) {
		return nil, fmt.Errorf("%w: the care period ended before this request was decided", careplan.ErrOfferingChangeInvalid)
	}
	if err := s.assertCareReaches(ctx, row.StudentID, effectiveFrom); err != nil {
		return nil, err
	}
	return &approvalContext{requestID: request.ID, phase: phase, selections: selections, effectiveFrom: effectiveFrom}, nil
}

func (s *OfferingChanges) requireCareOfferings(ctx context.Context) error {
	enabled, err := s.deps.Settings.CareOfferingsEnabled(ctx)
	if err != nil {
		return fmt.Errorf("offering change: resolve care offerings setting: %w", err)
	}
	if !enabled {
		return careplan.ErrCareOfferingsDisabled
	}
	return nil
}

func (s *OfferingChanges) assertCareReaches(ctx context.Context, studentID int64, effectiveFrom calendar.Date) error {
	student, err := s.deps.Students.LockStudent(ctx, studentID)
	if err != nil {
		return fmt.Errorf("offering change: load student for effective date: %w", err)
	}
	if student != nil && !student.EnrolledUntil.IsZero() && effectiveFrom.After(calendar.Date(student.EnrolledUntil)) {
		return fmt.Errorf("%w: care ends before the approved effective date", careplan.ErrOfferingChangeInvalid)
	}
	return nil
}

// checkApproval re-runs the preview's applicability and capacity checks on
// the approval date and reports whether the switch withdraws the child
// completely, which then needs the reviewer's confirmation.
func (s *OfferingChanges) checkApproval(
	ctx context.Context,
	row careplan.OfferingChangeRequest,
	approval *approvalContext,
	excluded map[int64]bool,
	confirmed bool,
) (bool, error) {
	allowCompleteWithdrawal, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return false, err
	}
	in := selectionMaterialization{
		phase: approval.phase, requestChildID: row.RequestChildID, effectiveFrom: approval.effectiveFrom,
		selections: approval.selections, excluded: excluded, allowCompleteWithdrawal: allowCompleteWithdrawal,
	}
	if err := s.assertApplicableAt(ctx, in, row.StudentID, &row); err != nil {
		return false, err
	}
	completeWithdrawal, err := s.completeWithdrawalAt(ctx, in)
	if err != nil {
		return false, err
	}
	if completeWithdrawal && !confirmed {
		return false, careplan.ErrCompleteWithdrawalConfirmationRequired
	}
	return completeWithdrawal, nil
}

func (s *OfferingChanges) completeWithdrawalAt(ctx context.Context, in selectionMaterialization) (bool, error) {
	current, err := s.currentSelections(ctx, in.requestChildID, in.effectiveFrom)
	if err != nil {
		return false, fmt.Errorf("offering change: list current offerings for withdrawal check: %w", err)
	}
	ids := bookedOfferingIDs(current)
	materialized, err := s.materializedSelections(ctx, in)
	if err != nil {
		return false, err
	}
	for _, selected := range materialized {
		ids = append(ids, selected.OfferingID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	offerings, err := s.offeringsByIDs(ctx, ids)
	if err != nil {
		return false, fmt.Errorf("offering change: list offerings for withdrawal check: %w", err)
	}
	byID := offeringsByID(offerings)
	return bookingsHaveCareDays(current, byID) && !selectionsHaveCareDays(materialized, byID), nil
}

// confirmedEffectiveFrom resolves the date an approval applies on. Without a
// date from the reviewer the guardian's date is moved forward. A date the
// reviewer confirmed is never moved: one outside the selectable range is
// refused (#2484).
func confirmedEffectiveFrom(confirmed *calendar.Date, requested, today calendar.Date, phase *ports.BookingPhase) (calendar.Date, error) {
	if confirmed == nil {
		return appliedOfferingChangeDateForPhase(requested, today, phase), nil
	}
	earliest := appliedOfferingChangeDateForPhase(today, today, phase)
	if confirmed.Before(earliest) {
		return calendar.Date(""), fmt.Errorf("%w: %s is before %s",
			careplan.ErrOfferingChangeDateOutOfRange, confirmed, earliest)
	}
	if phase != nil && confirmed.After(phase.ServiceEnd) {
		return calendar.Date(""), fmt.Errorf("%w: %s is after the care period ends on %s",
			careplan.ErrOfferingChangeDateOutOfRange, confirmed, phase.ServiceEnd)
	}
	return *confirmed, nil
}

func appliedOfferingChangeDate(effectiveFrom, today calendar.Date) calendar.Date {
	if effectiveFrom.Before(today) {
		return today
	}
	return effectiveFrom
}

func appliedOfferingChangeDateForPhase(effectiveFrom, today calendar.Date, phase *ports.BookingPhase) calendar.Date {
	effectiveFrom = appliedOfferingChangeDate(effectiveFrom, today)
	if phase != nil && effectiveFrom.Before(phase.ServiceStart) {
		return phase.ServiceStart
	}
	return effectiveFrom
}

// assertApplicableAt re-runs the checks an approval performs without
// writing: the selection has to be valid in the phase catalog on that date
// and every offering has to have a free slot. Preview and approval share it.
func (s *OfferingChanges) assertApplicableAt(ctx context.Context, in selectionMaterialization, studentID int64, pending *careplan.OfferingChangeRequest) error {
	validation := in
	validation.excluded = nil
	if _, err := s.validateSelections(ctx, validation); err != nil {
		return err
	}
	return s.assertCapacityAvailable(ctx, in, studentID, pending)
}

// offeringChangeAdjustmentReason writes the line the Änderungsprotokoll
// shows for an approved request. The generated prefix keeps its exact shape:
// migration 1.15.309 identifies request-applied rows by it. A date the office
// confirmed instead of the family's is named at the end (#2484).
func offeringChangeAdjustmentReason(row careplan.OfferingChangeRequest, requested calendar.Date, staffReason string) string {
	applied := offeringChangeEffectiveFrom(row)
	reason := fmt.Sprintf("Elternanfrage #%d freigegeben (gültig ab %s)", row.ID, applied.Format("02.01.2006"))
	if trimmed := strings.TrimSpace(staffReason); trimmed != "" {
		reason += ": " + trimmed
	}
	if requested != applied {
		reason += " · Wunschdatum der Eltern war " + requested.Format("02.01.2006")
	}
	return reason
}

func adjustmentChoicesOf(selections []careplan.OfferingChangeSelection) []careplan.OfferingAdjustmentChoice {
	choices := make([]careplan.OfferingAdjustmentChoice, 0, len(selections))
	for _, selected := range selections {
		choices = append(choices, careplan.OfferingAdjustmentChoice{
			OfferingID: selected.OfferingID, SelectedDays: append([]string(nil), selected.SelectedDays...),
		})
	}
	return choices
}

func cmpOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
