package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// pickupOfferingAdjustmentReason prefixes the Änderungsprotokoll line of a
// booking switch made from the permanent pickup-time editor.
const pickupOfferingAdjustmentReason = "Angebotswechsel wegen dauerhafter Gehzeiten"

// directAdjustmentScope is what a staff-side replacement validates against:
// the child's care period on the effective date and the live catalog there.
type directAdjustmentScope struct {
	*carePeriodScope
	effectiveFrom         calendar.Date
	catalog               *careplan.OfferingChangeCatalog
	bookingsAuthoritative bool
}

// PrepareDirectOfferingAdjustment takes the offering-derived write gates
// and validates the replacement, capacity included, before the caller locks
// the student.
func (s *OfferingChanges) PrepareDirectOfferingAdjustment(ctx context.Context, input careplan.DirectOfferingAdjustmentInput) error {
	if err := s.lockOfferingDerivedWrites(ctx); err != nil {
		return err
	}
	_, err := s.previewDirectOfferingAdjustment(ctx, input, true, s.todayDate())
	return err
}

// lockOfferingDerivedWrites serializes the correction against the other
// writers of the derived pickup and course rows before it validates.
func (s *OfferingChanges) lockOfferingDerivedWrites(ctx context.Context) error {
	return s.deps.Bookings.LockOfferingDerivedWrites(ctx)
}

// PreviewDirectOfferingAdjustment validates a staff-side replacement against
// the live offering catalog. Unlike the parent request it has no enablement
// or lead-time gate: staff correct the child's record directly, while the
// care-offerings feature itself must still be on.
func (s *OfferingChanges) PreviewDirectOfferingAdjustment(ctx context.Context, input careplan.DirectOfferingAdjustmentInput) (*careplan.DirectOfferingAdjustmentPreview, error) {
	return s.previewDirectOfferingAdjustment(ctx, input, false, s.todayDate())
}

func (s *OfferingChanges) previewDirectOfferingAdjustment(
	ctx context.Context,
	input careplan.DirectOfferingAdjustmentInput,
	checkCapacity bool,
	today calendar.Date,
) (*careplan.DirectOfferingAdjustmentPreview, error) {
	scope, err := s.directAdjustmentScope(ctx, input, today)
	if err != nil {
		return nil, err
	}
	result := &careplan.DirectOfferingAdjustmentPreview{
		RequestID: scope.period.RequestID, RequestChildID: scope.period.RequestChildID, Catalog: scope.catalog,
	}
	if input.Selections == nil {
		return result, nil
	}
	base, selected, err := s.directMaterializedSelections(ctx, input, scope, checkCapacity)
	if err != nil {
		return nil, err
	}
	diff, ids, err := s.directOfferingDiff(ctx, input, scope, base, selected)
	if err != nil {
		return nil, err
	}
	conflicts, err := s.manualPlanningConflicts(ctx, input.StudentID, diff)
	if err != nil {
		return nil, err
	}
	result.Consequences = &careplan.OfferingChangePreview{
		Selections: offeringPreviewSelections(ids, selected), ManualPlanningConflicts: conflicts,
		ArrivalExpectationsFollowBookings: scope.bookingsAuthoritative,
	}
	result.MaterializedPickupTimes, err = materializedPickupTimes(selected, scope.catalog)
	return result, err
}

func (s *OfferingChanges) directAdjustmentScope(ctx context.Context, input careplan.DirectOfferingAdjustmentInput, today calendar.Date) (*directAdjustmentScope, error) {
	if input.StudentID <= 0 {
		return nil, fmt.Errorf("%w: student is required", careplan.ErrOfferingChangeInvalid)
	}
	effectiveFrom := input.EffectiveFrom
	if effectiveFrom.IsZero() {
		effectiveFrom = today
	}
	if effectiveFrom.Before(today) {
		return nil, fmt.Errorf("%w: effective date is in the past", careplan.ErrOfferingChangeDateOutOfRange)
	}
	if err := s.requireCareOfferings(ctx); err != nil {
		return nil, err
	}
	bookingsAuthoritative, err := s.deps.Settings.BookingsAuthoritative(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering change: resolve booking authority for direct adjustment: %w", err)
	}
	scope, err := s.carePeriodAt(ctx, input.StudentID, effectiveFrom)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestContiguousApprovedCarePeriodEnd(ctx, input.StudentID, scope.period)
	if err != nil {
		return nil, err
	}
	earliest := today
	if earliest.Before(scope.phase.ServiceStart) {
		earliest = scope.phase.ServiceStart
	}
	catalog, err := s.catalogAt(ctx, scope, earliest, latest, effectiveFrom)
	return &directAdjustmentScope{
		carePeriodScope: scope, effectiveFrom: effectiveFrom, catalog: catalog, bookingsAuthoritative: bookingsAuthoritative,
	}, err
}

func (s *OfferingChanges) directMaterializedSelections(
	ctx context.Context,
	input careplan.DirectOfferingAdjustmentInput,
	scope *directAdjustmentScope,
	checkCapacity bool,
) ([]careplan.OfferingSelection, []careplan.OfferingSelection, error) {
	in := selectionMaterialization{
		phase: scope.phase, requestChildID: scope.period.RequestChildID, effectiveFrom: scope.effectiveFrom,
		selections: input.Selections, allowCompleteWithdrawal: scope.bookingsAuthoritative,
	}
	if _, err := s.validateSelections(ctx, in); err != nil {
		return nil, nil, err
	}
	excluded := offeringIDSet(input.ExcludedAutoOfferingIDs)
	if checkCapacity {
		capacity := in
		capacity.excluded = excluded
		if err := s.assertCapacityAvailable(ctx, capacity, input.StudentID, nil); err != nil {
			return nil, nil, err
		}
	}
	base, err := s.materializedSelections(ctx, in)
	if err != nil || len(excluded) == 0 {
		return base, base, err
	}
	in.excluded = excluded
	selected, err := s.materializedSelections(ctx, in)
	return base, selected, err
}

func (s *OfferingChanges) directOfferingDiff(
	ctx context.Context,
	input careplan.DirectOfferingAdjustmentInput,
	scope *directAdjustmentScope,
	base, selected []careplan.OfferingSelection,
) (*offeringDecisionDiff, []int64, error) {
	current, err := s.currentSelections(ctx, scope.period.RequestChildID, scope.effectiveFrom)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	ids, _, _ := offeringChangeSides(current, offeringChangeSelections(selected))
	diff, err := s.buildDecisionDiff(ctx, decisionDiffInput{
		excludedIDs: input.ExcludedAutoOfferingIDs, current: current, base: base, selected: selected,
		course: scope.catalog, requested: input.Selections,
	})
	if err != nil {
		return nil, nil, err
	}
	diff.phase = scope.phase
	diff.requested = input.Selections
	diff.effectiveFrom = scope.effectiveFrom
	return diff, ids, nil
}

// ApplyDirectOfferingAdjustment revalidates and then books the replacement
// through the dated offering adjustment, preserving its booking, capacity,
// cascade and append-only audit behaviour.
func (s *OfferingChanges) ApplyDirectOfferingAdjustment(ctx context.Context, input careplan.DirectOfferingAdjustmentInput) error {
	today := s.todayDate()
	preview, err := s.previewDirectOfferingAdjustment(ctx, input, true, today)
	if err != nil {
		return err
	}
	if input.Selections == nil {
		return fmt.Errorf("%w: selections are required", careplan.ErrOfferingChangeInvalid)
	}
	reason := pickupOfferingAdjustmentReason
	if trimmed := strings.TrimSpace(input.Reason); trimmed != "" {
		reason = pickupOfferingAdjustmentReason + ": " + trimmed
	}
	effectiveFrom := input.EffectiveFrom
	if effectiveFrom.IsZero() {
		effectiveFrom = today
	}
	_, err = s.deps.Bookings.AdjustOfferings(ctx, careplan.OfferingAdjustment{
		RequestID: preview.RequestID, ChildID: preview.RequestChildID, Offerings: adjustmentChoicesOf(input.Selections),
		ExcludedAutoAddTargetIDs: offeringIDSet(input.ExcludedAutoOfferingIDs), Reason: reason,
		ActorAccountID: input.ActorAccountID, ActorRole: input.ActorRole, EffectiveFrom: &effectiveFrom,
		CompleteWithdrawalConfirmed: input.CompleteWithdrawalConfirmed, Source: careplan.OfferingAdjustmentSourceDirect,
	})
	return err
}

// materializedPickupTimes projects the latest pickup time per care day the
// materialized selection books.
func materializedPickupTimes(selections []careplan.OfferingSelection, catalog *careplan.OfferingChangeCatalog) (map[string]string, error) {
	items := make(map[int64]careplan.OfferingChangeCatalogItem, len(catalog.Items))
	for _, item := range catalog.Items {
		items[item.OfferingID] = item
	}
	result := make(map[string]string)
	for _, selected := range selections {
		item, ok := items[selected.OfferingID]
		if !ok || !item.CountsAsCare {
			continue
		}
		if err := mergeMaterializedPickupTimes(result, item, selected.SelectedDays); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func mergeMaterializedPickupTimes(result map[string]string, item careplan.OfferingChangeCatalogItem, selectedDays []string) error {
	days := selectedDays
	if item.DaysOfWeekMode == daysOfWeekModeFixed {
		days = item.AvailableDays
	}
	for _, day := range days {
		value := strings.TrimSpace(item.PickupTimes[day])
		if value == "" {
			continue
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return fmt.Errorf("offering change: invalid pickup time %q for offering %d: %w", value, item.OfferingID, err)
		}
		current, currentErr := time.Parse("15:04", result[day])
		if result[day] == "" || currentErr != nil || current.Before(parsed) {
			result[day] = value
		}
	}
	return nil
}
