package application

import (
	"context"
	"fmt"
	"maps"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Catalog returns the offerings a guardian may choose from for the child, at
// the earliest date a change may take effect.
func (s *OfferingChanges) Catalog(ctx context.Context, studentID int64) (*careplan.OfferingChangeCatalog, error) {
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	scope, earliest, periods, err := s.carePeriodForEarliestEffectiveDate(ctx, studentID)
	if err != nil {
		return nil, err
	}
	latest := contiguousCarePeriodEnd(periods, *scope.period)
	return s.catalogAt(ctx, scope, earliest, latest, earliest)
}

// CatalogAt returns the offerings and booking state at a chosen effective
// date, so a complete replacement selection cannot be based on stale rows.
func (s *OfferingChanges) CatalogAt(ctx context.Context, studentID int64, effectiveFrom calendar.Date) (*careplan.OfferingChangeCatalog, error) {
	if effectiveFrom.IsZero() {
		return s.Catalog(ctx, studentID)
	}
	if err := s.changesEnabled(ctx); err != nil {
		return nil, err
	}
	earliest, err := s.EarliestEffectiveFrom(ctx)
	if err != nil {
		return nil, err
	}
	if effectiveFrom.Before(earliest) {
		return nil, fmt.Errorf("%w: effective date is earlier than the school's notice period", careplan.ErrOfferingChangeInvalid)
	}
	scope, err := s.carePeriodAt(ctx, studentID, effectiveFrom)
	if err != nil {
		return nil, err
	}
	if earliest.Before(scope.phase.ServiceStart) {
		earliest = scope.phase.ServiceStart
	}
	latest, err := s.latestContiguousApprovedCarePeriodEnd(ctx, studentID, scope.period)
	if err != nil {
		return nil, err
	}
	return s.catalogAt(ctx, scope, earliest, latest, effectiveFrom)
}

func (s *OfferingChanges) catalogAt(ctx context.Context, scope *carePeriodScope, earliest, latest, onDate calendar.Date) (*careplan.OfferingChangeCatalog, error) {
	phase, period := scope.phase, scope.period
	active, err := s.activeOfferings(ctx, phase.ID)
	if err != nil {
		return nil, fmt.Errorf("offering change: list active offerings: %w", err)
	}
	state, err := s.deps.Enrollment.CatalogState(ctx, phase.ID, period.RequestChildID, onDate, phase.ServiceEnd.AddDays(1))
	if err != nil {
		return nil, fmt.Errorf("offering change: list current offerings: %w", err)
	}
	activeByID := offeringsByID(active)
	allowed, err := s.catalogOfferings(ctx, activeByID, state.Current, period.TargetGradeLevel)
	if err != nil {
		return nil, err
	}
	catalog := &careplan.OfferingChangeCatalog{
		PhaseID: phase.ID, PhaseName: phase.Name, SelectionMode: phase.CareOfferingSelectionMode,
		EarliestEffectiveFrom: earliest, LatestEffectiveFrom: latest, CourseCapacityUntil: phase.ServiceEnd.AddDays(1),
		TargetGradeLevel: period.TargetGradeLevel, Items: make([]careplan.OfferingChangeCatalogItem, 0, len(allowed)),
	}
	if period.TargetSchoolClass != nil {
		catalog.TargetSchoolClass = *period.TargetSchoolClass
	}
	currentByID := make(map[int64]*careplan.BookedOffering, len(state.Current))
	for i := range state.Current {
		currentByID[state.Current[i].CareOfferingID] = &state.Current[i]
	}
	for _, offering := range allowed {
		item := catalogItem(offering, currentByID[offering.ID], state.CapacityPeaks[offering.ID])
		item.IsActive = activeByID[offering.ID] != nil
		catalog.Items = append(catalog.Items, item)
	}
	sort.SliceStable(catalog.Items, func(i, j int) bool {
		return catalog.Items[i].OfferingID < catalog.Items[j].OfferingID
	})
	return catalog, nil
}

// catalogOfferings is the active catalog the child's grade may book, plus
// every offering the child already holds: keeping the status quo is never
// blocked by a catalog edit.
func (s *OfferingChanges) catalogOfferings(
	ctx context.Context,
	activeByID map[int64]*careplan.CareOffering,
	current []careplan.BookedOffering,
	grade *int16,
) (map[int64]*careplan.CareOffering, error) {
	allowed, err := availableOfferingsForGrade(activeByID, grade)
	if err != nil {
		return nil, fmt.Errorf("offering change: filter eligible offerings: %w", err)
	}
	heldIDs := make([]int64, 0, len(current))
	for _, link := range current {
		if allowed[link.CareOfferingID] == nil {
			heldIDs = append(heldIDs, link.CareOfferingID)
		}
	}
	if len(heldIDs) == 0 {
		return allowed, nil
	}
	held, err := s.offeringsByIDs(ctx, heldIDs)
	if err != nil {
		return nil, fmt.Errorf("offering change: list held offerings: %w", err)
	}
	maps.Copy(allowed, offeringsByID(held))
	return allowed, nil
}

// availableOfferingsForGrade keeps the offerings whose availability rule
// admits the grade.
func availableOfferingsForGrade(catalog map[int64]*careplan.CareOffering, grade *int16) (map[int64]*careplan.CareOffering, error) {
	native, err := selectionCatalog(catalog)
	if err != nil {
		return nil, err
	}
	available := make(map[int64]*careplan.CareOffering, len(catalog))
	for id, offering := range catalog {
		matches, err := native[id].AvailabilityRule.MatchesGradeLevel(grade)
		if err != nil {
			return nil, fmt.Errorf("offering %d has invalid availability rule: %w", id, err)
		}
		if matches {
			available[id] = offering
		}
	}
	return available, nil
}

func catalogItem(offering *careplan.CareOffering, current *careplan.BookedOffering, taken int) careplan.OfferingChangeCatalogItem {
	item := careplan.OfferingChangeCatalogItem{
		OfferingID: offering.ID, Name: offering.Name, DaysOfWeekMode: offering.DaysOfWeekMode,
		AvailableDays: append([]string(nil), offering.AvailableDays...), SelectionGroup: offering.SelectionGroup,
		SelectionRule: offering.SelectionRule, IsRequired: offering.IsRequired, PriceCents: offering.PriceCents,
		IncludesLunch: offering.IncludesLunch, IncludesHoliday: offering.IncludesHolidayCare,
		CountsAsCare: offering.CountsAsCare, PickupTimes: maps.Clone(offering.PickupTimes),
		ActivityGroupID: offering.ActivityGroupID,
	}
	if offering.Description != nil {
		item.Description = *offering.Description
	}
	if current != nil {
		item.Selected = true
		item.SelectedDays = append([]string(nil), current.SelectedDays...)
		item.Automatic = len(current.ManualSelectedDays) == 0 && len(current.AutomaticSelectedDays) > 0
	}
	if offering.Capacity == nil {
		return item
	}
	capacity := *offering.Capacity
	item.Capacity = &capacity
	free := max(capacity-taken, 0)
	item.FreeSlots = &free
	return item
}

// The catalog reads below keep the listing order and the error texts of the
// catalog repository the review used to read through.

func (s *OfferingChanges) listOfferings(ctx context.Context, filter careplan.CareOfferingFilter, message string) ([]careplan.CareOffering, error) {
	values, err := s.deps.Catalog.ListCareOfferings(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", message, err)
	}
	return values, nil
}

func (s *OfferingChanges) activeOfferings(ctx context.Context, phaseID int64) ([]careplan.CareOffering, error) {
	return s.listOfferings(ctx, careplan.CareOfferingFilter{
		PhaseIDs: []int64{phaseID}, ActiveOnly: true, Order: careplan.OfferingOrderCatalog,
	}, "failed to list active offerings by phase")
}

func (s *OfferingChanges) phaseOfferings(ctx context.Context, phaseID int64) ([]careplan.CareOffering, error) {
	return s.listOfferings(ctx, careplan.CareOfferingFilter{
		PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog,
	}, "failed to list care offerings by phase")
}

func (s *OfferingChanges) offeringsByIDs(ctx context.Context, ids []int64) ([]careplan.CareOffering, error) {
	if len(ids) == 0 {
		return []careplan.CareOffering{}, nil
	}
	return s.listOfferings(ctx, careplan.CareOfferingFilter{
		IDs: ids, Order: careplan.OfferingOrderCatalog,
	}, "failed to list care offerings by ids")
}

func (s *OfferingChanges) lockOfferings(ctx context.Context, ids []int64) ([]careplan.CareOffering, error) {
	if len(ids) == 0 {
		return []careplan.CareOffering{}, nil
	}
	return s.listOfferings(ctx, careplan.CareOfferingFilter{
		IDs: ids, LockForUpdate: true, Order: careplan.OfferingOrderID,
	}, "failed to lock care offerings by ids")
}
