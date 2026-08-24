package enrollment

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

const pickupOfferingAdjustmentReason = "Angebotswechsel wegen dauerhafter Gehzeiten"

type directOfferingAdjustmentScope struct {
	period        *enrollmentModels.StudentCarePeriod
	phase         *enrollmentModels.Phase
	effectiveFrom timezone.Date
	catalog       *OfferingChangeCatalog
}

func (s *offeringChangeRequestService) PrepareDirectOfferingAdjustment(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
) error {
	if s.DirectApplier == nil {
		return fmt.Errorf("offering change: direct applier is not configured")
	}
	if err := s.DirectApplier.LockOfferingDerivedWrites(ctx); err != nil {
		return err
	}
	_, err := s.previewDirectOfferingAdjustment(ctx, input, true)
	return err
}

// PreviewDirectOfferingAdjustment validates a staff-side replacement against
// the live offering catalog. Unlike the parent request flow it intentionally
// has no request enablement or lead-time gate: staff are correcting the child
// record directly, while the care-offerings feature itself must still be on.
func (s *offeringChangeRequestService) PreviewDirectOfferingAdjustment(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
) (*DirectOfferingAdjustmentPreview, error) {
	return s.previewDirectOfferingAdjustment(ctx, input, false)
}

func (s *offeringChangeRequestService) previewDirectOfferingAdjustment(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
	checkCapacity bool,
) (*DirectOfferingAdjustmentPreview, error) {
	scope, err := s.directAdjustmentScope(ctx, input)
	if err != nil {
		return nil, err
	}
	result := &DirectOfferingAdjustmentPreview{
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
	result.Consequences, err = s.directOfferingConsequences(ctx, input.StudentID, diff, ids, selected)
	if err != nil {
		return nil, err
	}
	result.MaterializedPickupTimes, err = materializedPickupTimes(selected, scope.catalog)
	return result, err
}

func (s *offeringChangeRequestService) directAdjustmentScope(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
) (*directOfferingAdjustmentScope, error) {
	if input.StudentID <= 0 {
		return nil, fmt.Errorf("%w: student is required", ErrOfferingChangeInvalid)
	}
	effectiveFrom := input.EffectiveFrom
	if effectiveFrom.IsZero() {
		effectiveFrom = timezone.TodayDate()
	}
	if effectiveFrom.Before(timezone.TodayDate()) {
		return nil, fmt.Errorf("%w: effective date is in the past", ErrOfferingChangeDateOutOfRange)
	}
	if err := s.ensureDirectCareOfferingsEnabled(ctx); err != nil {
		return nil, err
	}
	period, phase, err := s.carePeriodAt(ctx, input.StudentID, effectiveFrom)
	if err != nil {
		return nil, err
	}
	latest, err := s.latestContiguousApprovedCarePeriodEnd(ctx, input.StudentID, period)
	if err != nil {
		return nil, err
	}
	earliest := timezone.TodayDate()
	if earliest.Before(phase.ServiceStartDate) {
		earliest = phase.ServiceStartDate
	}
	catalog, err := s.catalogAt(ctx, input.StudentID, period, phase, earliest, latest, effectiveFrom)
	return &directOfferingAdjustmentScope{period, phase, effectiveFrom, catalog}, err
}

func (s *offeringChangeRequestService) ensureDirectCareOfferingsEnabled(ctx context.Context) error {
	if s.Settings == nil {
		return fmt.Errorf("offering change: settings are required for direct adjustment")
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		return fmt.Errorf("offering change: resolve care offerings setting: %w", err)
	}
	if !enabled {
		return ErrCareOfferingsDisabled
	}
	return nil
}

func (s *offeringChangeRequestService) directMaterializedSelections(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
	scope *directOfferingAdjustmentScope,
	checkCapacity bool,
) ([]materializedOfferingSelection, []materializedOfferingSelection, error) {
	excluded := offeringIDSet(input.ExcludedAutoOfferingIDs)
	if _, err := s.validateSelections(
		ctx, scope.phase, scope.period.RequestChildID, scope.effectiveFrom, input.Selections,
	); err != nil {
		return nil, nil, err
	}
	if checkCapacity {
		if err := s.assertCapacityAvailable(
			ctx, scope.phase, scope.period.RequestChildID, scope.effectiveFrom, input.Selections, excluded,
		); err != nil {
			return nil, nil, err
		}
	}
	base, err := s.materializedSelections(
		ctx, scope.phase, scope.period.RequestChildID, scope.effectiveFrom, input.Selections,
	)
	if err != nil || len(excluded) == 0 {
		return base, base, err
	}
	selected, err := s.materializedSelectionsExcluding(
		ctx, scope.phase, scope.period.RequestChildID, scope.effectiveFrom, input.Selections, excluded,
	)
	return base, selected, err
}

func (s *offeringChangeRequestService) directOfferingDiff(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
	scope *directOfferingAdjustmentScope,
	base, selected []materializedOfferingSelection,
) (*offeringDecisionDiff, []int64, error) {
	current, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(
		ctx, scope.period.RequestChildID, scope.effectiveFrom,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	ids, currentByID, requestedByID := offeringChangeSides(current, offeringChangeSelections(selected))
	diff, err := s.buildDecisionDiff(
		ctx, input.ExcludedAutoOfferingIDs, current, base, selected, ids, currentByID, requestedByID,
	)
	if err != nil {
		return nil, nil, err
	}
	diff.phase = scope.phase
	diff.requested = input.Selections
	diff.effectiveFrom = scope.effectiveFrom
	return diff, ids, nil
}

func (s *offeringChangeRequestService) directOfferingConsequences(
	ctx context.Context,
	studentID int64,
	diff *offeringDecisionDiff,
	ids []int64,
	selected []materializedOfferingSelection,
) (*OfferingChangePreview, error) {
	conflicts, err := s.manualPlanningConflicts(ctx, studentID, diff)
	if err != nil {
		return nil, err
	}
	arrivalFollowsBookings, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentBookingsAuthoritative)
	if err != nil {
		return nil, fmt.Errorf("offering change: resolve booking authority for preview: %w", err)
	}
	return &OfferingChangePreview{
		Selections: offeringPreviewSelections(ids, selected), ManualPlanningConflicts: conflicts,
		ArrivalExpectationsFollowBookings: arrivalFollowsBookings,
	}, nil
}

func offeringPreviewSelections(
	ids []int64,
	selected []materializedOfferingSelection,
) []OfferingChangePreviewSelection {
	selectedByID := materializedSelectionPointers(selected)
	previewSelections := make([]OfferingChangePreviewSelection, 0, len(ids))
	for _, offeringID := range ids {
		selection := selectedByID[offeringID]
		if selection == nil {
			previewSelections = append(previewSelections, OfferingChangePreviewSelection{
				OfferingID: offeringID,
				State:      "removed",
			})
			continue
		}
		previewSelections = append(previewSelections, OfferingChangePreviewSelection{
			OfferingID: offeringID,
			State:      "booked",
			Days:       slices.Clone(selection.SelectedDays),
		})
	}
	return previewSelections
}

// ApplyDirectOfferingAdjustment revalidates and then delegates the write to
// DecisionService.UpdateChildOfferings, preserving the established dated
// booking, capacity, cascade and append-only offering audit behavior.
func (s *offeringChangeRequestService) ApplyDirectOfferingAdjustment(
	ctx context.Context,
	input DirectOfferingAdjustmentInput,
) error {
	preview, err := s.previewDirectOfferingAdjustment(ctx, input, true)
	if err != nil {
		return err
	}
	if input.Selections == nil {
		return fmt.Errorf("%w: selections are required", ErrOfferingChangeInvalid)
	}
	if s.DirectApplier == nil {
		return fmt.Errorf("offering change: direct applier is not configured")
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		reason = pickupOfferingAdjustmentReason
	} else {
		reason = pickupOfferingAdjustmentReason + ": " + reason
	}
	excluded := offeringIDSet(input.ExcludedAutoOfferingIDs)
	effectiveFrom := input.EffectiveFrom
	if effectiveFrom.IsZero() {
		effectiveFrom = timezone.TodayDate()
	}
	_, err = s.DirectApplier.UpdateChildOfferings(ctx, UpdateChildOfferingsInput{
		RequestID:                   preview.RequestID,
		ChildID:                     preview.RequestChildID,
		Offerings:                   adjustmentSelections(input.Selections),
		Reason:                      reason,
		ActorAccountID:              input.ActorAccountID,
		ActorRole:                   input.ActorRole,
		EffectiveFrom:               &effectiveFrom,
		ExcludedAutoAddTargetIDs:    excluded,
		CompleteWithdrawalConfirmed: input.CompleteWithdrawalConfirmed,
	})
	return err
}

func materializedPickupTimes(
	selections []materializedOfferingSelection,
	catalog *OfferingChangeCatalog,
) (map[string]string, error) {
	items := make(map[int64]OfferingChangeCatalogItem, len(catalog.Items))
	for _, item := range catalog.Items {
		items[item.OfferingID] = item
	}
	result := make(map[string]string)
	for _, selection := range selections {
		item, ok := items[selection.OfferingID]
		if !ok || !item.CountsAsCare {
			continue
		}
		if err := mergeMaterializedPickupTimes(result, item, selection.SelectedDays); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func mergeMaterializedPickupTimes(
	result map[string]string,
	item OfferingChangeCatalogItem,
	selectedDays []string,
) error {
	days := selectedDays
	if item.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
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
