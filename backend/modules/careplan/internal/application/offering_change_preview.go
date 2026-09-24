package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PreviewDecision materializes a pending approval with the supplied
// Mitbuchungs-Regel overrides and confirmed date without writing it. The
// preview is what the office decides from, so it fails on everything the
// approval would fail on, at the same date (#2484).
func (s *OfferingChanges) PreviewDecision(
	ctx context.Context,
	requestID int64,
	excludedIDs []int64,
	effectiveFrom *calendar.Date,
) (*careplan.OfferingChangePreview, error) {
	row, err := s.previewableRow(ctx, requestID)
	if err != nil {
		return nil, err
	}
	diff, err := s.decisionDiff(ctx, row, excludedIDs, effectiveFrom)
	if err != nil {
		return nil, err
	}
	allowCompleteWithdrawal, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return nil, err
	}
	in := selectionMaterialization{
		phase: diff.phase, requestChildID: row.RequestChildID, effectiveFrom: diff.effectiveFrom,
		selections: diff.requested, excluded: offeringIDSet(excludedIDs), allowCompleteWithdrawal: allowCompleteWithdrawal,
	}
	if err := s.assertApplicableAt(ctx, in, row.StudentID, &row); err != nil {
		return nil, err
	}
	ids, _, _ := offeringChangeSides(diff.current, offeringChangeSelections(diff.base))
	conflicts, err := s.manualPlanningConflicts(ctx, row.StudentID, diff)
	if err != nil {
		return nil, err
	}
	arrivalExpectationsFollowBookings, err := s.deps.Settings.BookingsAuthoritative(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering change: resolve booking authority for preview: %w", err)
	}
	return &careplan.OfferingChangePreview{
		Selections:                        offeringPreviewSelections(ids, diff.selected),
		ManualPlanningConflicts:           conflicts,
		ArrivalExpectationsFollowBookings: arrivalExpectationsFollowBookings,
	}, nil
}

// previewableRow reads a pending request of a child the caller may review.
func (s *OfferingChanges) previewableRow(ctx context.Context, requestID int64) (careplan.OfferingChangeRequest, error) {
	if requestID <= 0 {
		return careplan.OfferingChangeRequest{}, fmt.Errorf("%w: request is required", careplan.ErrOfferingChangeInvalid)
	}
	row, err := s.deps.Rows.Find(ctx, requestID)
	if err != nil {
		return careplan.OfferingChangeRequest{}, err
	}
	if offeringChangeTerminal(row) {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeNotPending
	}
	student, err := s.deps.Students.FindStudent(ctx, row.StudentID)
	if err != nil {
		return careplan.OfferingChangeRequest{}, fmt.Errorf("offering change: load student for preview: %w", err)
	}
	if student == nil || student.Alumnus || careEnded(student, s.todayDate()) {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeNotFound
	}
	allowed, err := s.reviewAllows(ctx, student)
	if err != nil {
		return careplan.OfferingChangeRequest{}, err
	}
	if !allowed {
		return careplan.OfferingChangeRequest{}, careplan.ErrOfferingChangeForbidden
	}
	return row, nil
}

func offeringPreviewSelections(ids []int64, selected []careplan.OfferingSelection) []careplan.OfferingChangePreviewSelection {
	selectedByID := selectionPointersByOffering(selected)
	preview := make([]careplan.OfferingChangePreviewSelection, 0, len(ids))
	for _, offeringID := range ids {
		selection := selectedByID[offeringID]
		if selection == nil {
			preview = append(preview, careplan.OfferingChangePreviewSelection{OfferingID: offeringID, State: "removed"})
			continue
		}
		preview = append(preview, careplan.OfferingChangePreviewSelection{
			OfferingID: offeringID, State: "booked", Days: slices.Clone(selection.SelectedDays),
		})
	}
	return preview
}

// manualPlanningConflicts groups the future manual planning of the child
// the proposed bookings no longer cover.
func (s *OfferingChanges) manualPlanningConflicts(ctx context.Context, studentID int64, diff *offeringDecisionDiff) ([]careplan.ManualPlanningConflict, error) {
	occurrences, err := s.deps.Planning.ListManualPlanningOccurrences(
		ctx, studentID, diff.effectiveFrom.String(), diff.phase.ServiceEnd.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("offering change: list manual planning conflicts: %w", err)
	}
	return aggregateManualPlanningConflicts(occurrences, diff), nil
}

func proposedCareCoversOccurrence(diff *offeringDecisionDiff, occurrence ports.ManualPlanningOccurrence) bool {
	for _, selected := range diff.selected {
		if proposedSelectionCoversOccurrence(diff, selected, occurrence) {
			return true
		}
	}
	return false
}

func proposedSelectionCoversOccurrence(diff *offeringDecisionDiff, selected careplan.OfferingSelection, occurrence ports.ManualPlanningOccurrence) bool {
	date := calendar.Date(occurrence.Date)
	if diff == nil || diff.phase == nil || date.Before(diff.effectiveFrom) || date.After(diff.phase.ServiceEnd) {
		return false
	}
	offering := diff.offeringByID[selected.OfferingID]
	if offering == nil || !offering.CountsAsCare {
		return false
	}
	days := selected.SelectedDays
	if offering.DaysOfWeekMode == daysOfWeekModeFixed {
		days = offering.AvailableDays
	}
	return slices.Contains(days, canonicalDayForWeekday(date.Weekday()))
}

func proposedLegacyPlanningCoversOccurrence(diff *offeringDecisionDiff, occurrence ports.ManualPlanningOccurrence) bool {
	if diff == nil {
		return false
	}
	for _, selected := range diff.selected {
		offering := diff.offeringByID[selected.OfferingID]
		if offering == nil || offering.ActivityGroupID == nil || *offering.ActivityGroupID != occurrence.ActivityGroupID {
			continue
		}
		if proposedSelectionCoversOccurrence(diff, selected, occurrence) {
			return true
		}
	}
	return false
}

func aggregateManualPlanningConflicts(occurrences []ports.ManualPlanningOccurrence, diff *offeringDecisionDiff) []careplan.ManualPlanningConflict {
	conflicts := make([]careplan.ManualPlanningConflict, 0)
	groupIndexes := make(map[int64]int)
	seenDays := make(map[int64]map[string]bool)
	for _, occurrence := range occurrences {
		if proposedLegacyPlanningCoversOccurrence(diff, occurrence) || proposedCareCoversOccurrence(diff, occurrence) {
			continue
		}
		date := calendar.Date(occurrence.Date)
		groupIndex, exists := groupIndexes[occurrence.ActivityGroupID]
		if !exists {
			conflicts = append(conflicts, careplan.ManualPlanningConflict{
				ActivityGroupID: occurrence.ActivityGroupID, ActivityGroupName: occurrence.ActivityGroupName, FirstDate: date,
			})
			groupIndex = len(conflicts) - 1
			groupIndexes[occurrence.ActivityGroupID] = groupIndex
			seenDays[occurrence.ActivityGroupID] = make(map[string]bool)
		}
		conflict := &conflicts[groupIndex]
		conflict.OccurrenceCount++
		if date.Before(conflict.FirstDate) {
			conflict.FirstDate = date
		}
		if day := canonicalDayForWeekday(date.Weekday()); !seenDays[occurrence.ActivityGroupID][day] {
			seenDays[occurrence.ActivityGroupID][day] = true
			conflict.Days = append(conflict.Days, day)
		}
	}
	for i := range conflicts {
		conflicts[i].Days = canonicalDays(conflicts[i].Days)
	}
	return conflicts
}

func canonicalDayForWeekday(weekday time.Weekday) string {
	return [...]string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}[weekday]
}
