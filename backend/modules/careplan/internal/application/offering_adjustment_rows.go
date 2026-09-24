package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// scheduledOfferingReplacement is a future dated selection that must survive
// an undated correction.
type scheduledOfferingReplacement struct {
	EffectiveFrom calendar.Date
	Rows          []*careplan.BookedOffering
}

func (m *BookingMaterialization) persistOfferingAdjustment(ctx context.Context, work *offeringAdjustmentWork) error {
	scheduled, err := m.scheduledOfferingReplacements(ctx, work.child.ID, work.selectionDate, work.effectiveFrom)
	if err != nil {
		return err
	}
	if err := m.persistOfferingBookings(ctx, work, scheduled); err != nil {
		return err
	}
	materializeFrom := work.effectiveFrom
	if materializeFrom == nil && len(scheduled) > 0 {
		materializeFrom = &work.selectionDate
	}
	studentID := *work.child.CreatedStudentID
	adjusted := adjustedRoster{requestChildID: work.child.ID, studentID: studentID, phase: work.phase.OfferingPhase}
	if err := m.rematerializeAdjustedEnrollments(ctx, adjusted, work.beforeLinks, work.replacement, materializeFrom); err != nil {
		return err
	}
	for _, future := range scheduled {
		if err := m.splitAdjustedEnrollments(ctx, adjusted, future.Rows, future.EffectiveFrom); err != nil {
			return err
		}
	}
	return nil
}

// scheduledOfferingReplacements captures future changes before an undated
// staff correction rewrites the current selection. A correction applies now;
// it must not silently cancel a separately approved future request.
func (m *BookingMaterialization) scheduledOfferingReplacements(
	ctx context.Context,
	requestChildID int64,
	selectionDate calendar.Date,
	effectiveFrom *calendar.Date,
) ([]scheduledOfferingReplacement, error) {
	if effectiveFrom != nil {
		return nil, nil
	}
	history, err := m.deps.Enrollment.SelectionHistory(ctx, requestChildID)
	if err != nil {
		return nil, fmt.Errorf("decision: list scheduled child offerings: %w", err)
	}
	byDate := make(map[calendar.Date][]*careplan.BookedOffering)
	for _, row := range bookedOfferingPointers(history) {
		if row.ValidFrom == nil || !row.ValidFrom.After(selectionDate) {
			continue
		}
		byDate[*row.ValidFrom] = append(byDate[*row.ValidFrom], row)
	}
	dates := make([]calendar.Date, 0, len(byDate))
	for date := range byDate {
		dates = append(dates, date)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	scheduled := make([]scheduledOfferingReplacement, 0, len(dates))
	for _, date := range dates {
		scheduled = append(scheduled, scheduledOfferingReplacement{EffectiveFrom: date, Rows: byDate[date]})
	}
	return scheduled, nil
}

// persistOfferingBookings writes the new effective bookings: dated when the
// switch has a date or future selections must be kept, a replacement
// otherwise.
func (m *BookingMaterialization) persistOfferingBookings(ctx context.Context, work *offeringAdjustmentWork, scheduled []scheduledOfferingReplacement) error {
	phase := work.phase.OfferingPhase
	if work.effectiveFrom == nil && len(scheduled) == 0 {
		if err := m.changeCareBookings(ctx, work.child.ID, phase, nil, work.replacement); err != nil {
			return fmt.Errorf("decision: replace child offerings: %w", err)
		}
		return nil
	}
	if err := m.changeCareBookings(ctx, work.child.ID, phase, &work.selectionDate, work.replacement); err != nil {
		return fmt.Errorf("decision: schedule child offerings: %w", err)
	}
	for _, future := range scheduled {
		if err := m.changeCareBookings(ctx, work.child.ID, phase, &future.EffectiveFrom, future.Rows); err != nil {
			return fmt.Errorf("decision: restore scheduled child offerings: %w", err)
		}
	}
	return nil
}

// changeCareBookings sends the selection to Care Plan's effective bookings,
// bounded by the phase's service window, and stamps the validity it wrote
// back onto each selection so the roster drafts use the same window.
func (m *BookingMaterialization) changeCareBookings(
	ctx context.Context,
	childID int64,
	phase careplan.OfferingPhase,
	effectiveFrom *calendar.Date,
	selections []*careplan.BookedOffering,
) error {
	start, until := phase.ServiceStart, phase.ServiceEnd.AddDays(1)
	bookings := make([]careplan.CareOfferingBooking, 0, len(selections))
	for _, selected := range selections {
		if selected == nil {
			return errors.New("care booking cannot be nil")
		}
		from, end := start, until
		if selected.ValidFrom != nil {
			from = *selected.ValidFrom
		}
		if effectiveFrom != nil {
			from = *effectiveFrom
		}
		if selected.ValidUntil != nil && (effectiveFrom == nil || selected.ValidUntil.Before(until)) {
			end = *selected.ValidUntil
		}
		manual := selected.ManualSelectedDays
		if len(manual) == 0 && len(selected.AutomaticSelectedDays) == 0 {
			manual = selected.SelectedDays
		}
		bookingFrom, bookingUntil := careplan.Date(from), careplan.Date(end)
		bookings = append(bookings, careplan.CareOfferingBooking{
			CareOfferingID: selected.CareOfferingID, ManualSelectedDays: manual,
			AutomaticSelectedDays: selected.AutomaticSelectedDays,
			ValidFrom:             &bookingFrom, ValidUntil: &bookingUntil,
		})
		selected.ValidFrom, selected.ValidUntil = &from, &end
	}
	if effectiveFrom != nil {
		return m.deps.Bookings.ScheduleCareOfferingBookings(ctx, childID, careplan.Date(*effectiveFrom), bookings)
	}
	return m.deps.Bookings.ReplaceCareOfferingBookings(ctx, childID, bookings)
}

// adjustedRoster names the roster rows an adjustment rewrites: those one
// request child materialized for its student within the phase.
type adjustedRoster struct {
	requestChildID int64
	studentID      int64
	phase          careplan.OfferingPhase
}

func (m *BookingMaterialization) rematerializeAdjustedEnrollments(
	ctx context.Context,
	adjusted adjustedRoster,
	beforeLinks []*careplan.BookedOffering,
	replacement []*careplan.BookedOffering,
	effectiveFrom *calendar.Date,
) error {
	if err := m.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	if len(beforeLinks) > 0 {
		if err := m.backfillLegacyAdjustedEnrollments(ctx, adjusted, beforeLinks); err != nil {
			return err
		}
	}
	if effectiveFrom != nil {
		return m.splitAdjustedEnrollments(ctx, adjusted, replacement, *effectiveFrom)
	}
	previousGroups, err := m.enrollmentGroupIDsForRequestChild(ctx, adjusted)
	if err != nil {
		return err
	}
	if err := m.deps.Rosters.DeleteRequestChildEnrollments(ctx, adjusted.studentID, adjusted.requestChildID); err != nil {
		return fmt.Errorf("decision: delete sourced adjusted enrollments: %w", err)
	}
	if err := m.materializeEnrollmentsFrom(ctx, adjusted.requestChildID, adjusted.studentID, adjusted.phase, nil, true); err != nil {
		return err
	}
	// The materialization reconciled the templates the NEW selection plans;
	// templates only the OLD selection planned still carry the child on
	// already-materialized future occurrences (#2147 review).
	return m.reconcileEnrollmentInstanceRosters(ctx, adjusted.studentID, previousGroups, m.enrollmentRewriteBoundary(nil))
}

// enrollmentGroupIDsForRequestChild returns the activity groups the child's
// tagged enrollment rows currently reference, so a full rematerialization can
// reconcile the occurrences of templates the new selection no longer plans.
func (m *BookingMaterialization) enrollmentGroupIDsForRequestChild(ctx context.Context, adjusted adjustedRoster) (map[int64]bool, error) {
	rows, err := m.deps.Rosters.StudentEnrollments(ctx, adjusted.studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: list enrollments for adjustment reconcile: %w", err)
	}
	groupIDs := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if taggedWithRequestChild(row, adjusted.requestChildID) {
			groupIDs[row.ActivityGroupID] = true
		}
	}
	return groupIDs, nil
}

func taggedWithRequestChild(row ports.RosterEnrollment, requestChildID int64) bool {
	return row.EnrollmentRequestChildID != nil && *row.EnrollmentRequestChildID == requestChildID
}

// splitAdjustedEnrollments applies the new offering selection from
// effectiveFrom onward without rewriting the past. Deleting and re-creating
// the whole phase window (what an admin correction does) would erase the
// record of which groups the child actually attended before the switch, and
// the attendance taken there would no longer have a roster row behind it.
//
// Per existing row materialized from this request child:
//   - already ended before the switch: untouched, it is history
//   - identical to a row the new selection wants: kept, so an unchanged
//     offering does not become two adjacent rows
//   - starts on/after the switch: deleted, it never took effect
//   - started earlier: capped at effectiveFrom (valid_until is exclusive, so
//     the last attended day is the day before)
//
// Rows the new selection still wants are then materialized starting at the
// switch date. Phase-window edits are deliberately not reconciled here: a
// kept row keeps its original valid_until, and correcting phase dates is what
// the undated (correction) path is for.
func (m *BookingMaterialization) splitAdjustedEnrollments(
	ctx context.Context,
	adjusted adjustedRoster,
	replacement []*careplan.BookedOffering,
	effectiveFrom calendar.Date,
) error {
	drafts, multiSource, err := m.careEnrollmentDraftsForLinks(ctx, adjusted.requestChildID, adjusted.studentID, replacement, adjusted.phase)
	if err != nil {
		return err
	}
	existing, err := m.deps.Rosters.StudentEnrollments(ctx, adjusted.studentID)
	if err != nil {
		return fmt.Errorf("decision: list enrollments for dated adjustment: %w", err)
	}
	// Every template the switch touches — rows it caps or deletes as well as
	// rows it creates — must afterwards be reconciled onto its already-
	// materialized future occurrences (#2147 review).
	affectedGroups := draftGroupIDSet(drafts)
	for _, row := range existing {
		if !taggedWithRequestChild(row, adjusted.requestChildID) {
			continue
		}
		affectedGroups[row.ActivityGroupID] = true
		if err := m.reconcileAdjustedEnrollment(ctx, row, adjusted.phase, effectiveFrom, drafts); err != nil {
			return err
		}
	}
	if err := m.persistCareEnrollmentDrafts(ctx, adjusted.requestChildID, adjusted.studentID, adjusted.phase, drafts, &effectiveFrom); err != nil {
		return err
	}
	boundary := m.enrollmentRewriteBoundary(&effectiveFrom)
	if err := m.reconcileEnrollmentInstanceRosters(ctx, adjusted.studentID, affectedGroups, boundary); err != nil {
		return err
	}
	// Multi-source templates were deliberately not drafted (see
	// addSourcedTemplateDrafts); the resync re-establishes the child's union
	// coverage from the switch date onward — including templates the child
	// keeps through ANOTHER of the template's source offerings after leaving
	// this one. Scoped to this child: a dated switch must not reconcile other
	// children's rows as a side effect.
	return m.resyncMultiSourceTemplates(ctx, multiSource, boundary, []int64{adjusted.requestChildID})
}

// reconcileAdjustedEnrollment keeps, caps or deletes one of the request
// child's rows at a dated switch.
func (m *BookingMaterialization) reconcileAdjustedEnrollment(
	ctx context.Context,
	row ports.RosterEnrollment,
	phase careplan.OfferingPhase,
	effectiveFrom calendar.Date,
	drafts map[int64]*careEnrollmentDraft,
) error {
	if row.ValidUntil != nil && !row.ValidUntil.After(effectiveFrom) {
		return nil
	}
	if draft := drafts[row.ActivityGroupID]; draft != nil && !row.ValidFrom.After(effectiveFrom) && careDraftMatchesEnrollment(draft, row) {
		// The retained row is only ever extended to the end the draft itself
		// may reach — the phase end, clamped by the sourced segment's envelope
		// and the offering-link window (#2147 review). Extending a capped
		// split predecessor back to the phase end would restore coverage past
		// the split and overlap its successor.
		draftEndExclusive := careDraftValidUntil(draft, phase)
		if row.ValidUntil != nil && row.ValidUntil.Before(draftEndExclusive) {
			if err := m.deps.Rosters.SetEnrollmentValidUntil(ctx, row.ID, draftEndExclusive); err != nil {
				return fmt.Errorf("decision: extend retained adjusted enrollment: %w", err)
			}
		}
		delete(drafts, row.ActivityGroupID)
		return nil
	}
	if !row.ValidFrom.Before(effectiveFrom) {
		if err := m.deps.Rosters.DeleteEnrollment(ctx, row.ID); err != nil {
			return fmt.Errorf("decision: delete not-yet-effective adjusted enrollment: %w", err)
		}
		return nil
	}
	if err := m.deps.Rosters.SetEnrollmentValidUntil(ctx, row.ID, effectiveFrom); err != nil {
		return fmt.Errorf("decision: cap adjusted enrollment: %w", err)
	}
	return nil
}

// backfillLegacyAdjustedEnrollments stamps the request child on the rows an
// approval materialized before the provenance column existed, so the
// rewrite below finds them.
func (m *BookingMaterialization) backfillLegacyAdjustedEnrollments(ctx context.Context, adjusted adjustedRoster, beforeLinks []*careplan.BookedOffering) error {
	offerings, err := m.catalog.listByIDs(ctx, bookedOfferingIDs(beforeLinks))
	if err != nil {
		return fmt.Errorf("decision: list existing child offerings for legacy enrollment cleanup: %w", err)
	}
	groupIDs := make([]int64, 0, len(offerings))
	seen := make(map[int64]bool, len(offerings))
	for _, offering := range offerings {
		if offering.ActivityGroupID == nil || *offering.ActivityGroupID <= 0 || seen[*offering.ActivityGroupID] {
			continue
		}
		seen[*offering.ActivityGroupID] = true
		groupIDs = append(groupIDs, *offering.ActivityGroupID)
	}
	if len(groupIDs) == 0 {
		return nil
	}
	if err := m.deps.Rosters.BackfillRequestChildSource(ctx, adjusted.studentID, adjusted.requestChildID, groupIDs); err != nil {
		return fmt.Errorf("decision: backfill legacy adjusted enrollments: %w", err)
	}
	return nil
}

func (m *BookingMaterialization) recordOfferingAdjustment(ctx context.Context, work *offeringAdjustmentWork) (int64, error) {
	actorName, actorEmail := m.deps.Audit.ActorSnapshot(ctx, work.input.ActorAccountID)
	actorRole := strings.TrimSpace(work.input.ActorRole)
	if actorRole == "" {
		actorRole = "admin"
	}
	work.input.ActorRole = actorRole
	id, err := m.deps.Audit.RecordAdjustment(ctx, careplan.OfferingAdjustmentRecord{
		RequestID:                   work.child.RequestID,
		RequestChildID:              work.child.ID,
		StudentID:                   *work.child.CreatedStudentID,
		ActorAccountID:              work.input.ActorAccountID,
		ActorRole:                   actorRole,
		ActorNameSnapshot:           actorName,
		ActorEmailSnapshot:          actorEmail,
		Reason:                      work.reason,
		Source:                      work.input.Source,
		Before:                      work.beforeJSON,
		After:                       work.afterJSON,
		CompleteWithdrawalConfirmed: work.isCompleteWithdrawal,
	})
	if err != nil {
		return 0, fmt.Errorf("decision: create offering adjustment audit: %w", err)
	}
	return id, nil
}

func (m *BookingMaterialization) reconcileOfferingAdjustmentWithdrawal(ctx context.Context, work *offeringAdjustmentWork, adjustmentID int64) error {
	if m.deps.Withdrawals == nil {
		return nil
	}
	err := m.deps.Withdrawals.ReconcileAuthoritativeBookingChange(ctx, careplan.CareWithdrawalBookingChange{
		StudentID: *work.child.CreatedStudentID, FirstBookinglessDay: work.selectionDate,
		WasCompleteWithdrawal: work.isCompleteWithdrawal,
		SourceAdjustmentID:    adjustmentID, SourceRequestChildID: work.child.ID, ConfirmedBy: work.input.ActorAccountID,
		ConfirmedRole: work.input.ActorRole, SourceOfferings: careExitSourceOfferingsFromLinks(work.beforeLinks, work.offeringByID),
	})
	if err != nil {
		return fmt.Errorf("offering adjustment: reconcile complete withdrawal: %w", err)
	}
	return nil
}
