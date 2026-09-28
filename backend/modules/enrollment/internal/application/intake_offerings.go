package application

import (
	"context"
	"errors"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The intake validates and materializes a child's offering picks through
// Enrollment's selection engine and records the result as submitted choices
// and Care Plan bookings.

// currentOfferingSelectionDate is the day a child's stored offering links are
// read at to yield the booking in force now. A dated change splits the links
// into intervals, so reading them without a date or at the service start
// would mix the superseded booking into "the current selection".
func (s *Intake) currentOfferingSelectionDate(phase *enrollment.Phase) calendar.Date {
	return offeringSelectionDateOn(phase, calendar.TodayDate())
}

// nativeOfferingCatalog describes enrollment offering rows in the terms of
// the selection engine.
func nativeOfferingCatalog(catalog map[int64]*enrollmentModels.CareOffering) map[int64]*selection.Offering {
	result := make(map[int64]*selection.Offering, len(catalog))
	for id, offering := range catalog {
		if offering == nil {
			continue
		}
		native := &selection.Offering{
			ID: offering.ID, SortOrder: offering.SortOrder, DaysOfWeekMode: offering.DaysOfWeekMode,
			AvailableDays: offering.AvailableDays, CountsAsCare: offering.CountsAsCare, IncludesLunch: offering.IncludesLunch,
			IsRequired: offering.IsRequired, SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule,
			AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, AutoAddGradeLevels: offering.AutoAddGradeLevels,
		}
		if rule := offering.AvailabilityRule; rule != nil {
			native.AvailabilityRule = &selection.AvailabilityRule{Match: rule.Match, Conditions: make([]selection.AvailabilityCondition, len(rule.Conditions))}
			for i, condition := range rule.Conditions {
				native.AvailabilityRule.Conditions[i] = selection.AvailabilityCondition{Source: condition.Source, Operator: condition.Operator, Value: append([]int(nil), condition.Value...)}
			}
		}
		result[id] = native
	}
	return result
}

func nativeSelectionChild(child SubmitChild) selection.Child {
	native := selection.Child{TargetGradeLevel: child.TargetGradeLevel, OfferingIDs: child.OfferingIDs, ExcludedAutoAddTargetIDs: child.ExcludedAutoAddTargetIDs}
	if child.OfferingDays != nil {
		native.OfferingDays = make([]selection.DaySelection, len(child.OfferingDays))
		for j, row := range child.OfferingDays {
			native.OfferingDays[j] = selection.DaySelection{OfferingID: row.OfferingID, SelectedDays: row.SelectedDays}
		}
	}
	return native
}

// applyNativeSelection copies the selection engine's normalized picks back
// onto the submitted child.
func applyNativeSelection(child *SubmitChild, native selection.Child) {
	child.OfferingIDs = native.OfferingIDs
	if native.OfferingDays == nil {
		child.OfferingDays = nil
		return
	}
	child.OfferingDays = make([]SubmitOfferingDays, len(native.OfferingDays))
	for j, row := range native.OfferingDays {
		child.OfferingDays[j] = SubmitOfferingDays{OfferingID: row.OfferingID, SelectedDays: row.SelectedDays}
	}
}

// materializeChildrenOfferingSelections validates every child's picks
// against the open catalog and replaces them with their materialization,
// including the automatic shares.
func materializeChildrenOfferingSelections(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering, selectionMode string) ([][]selection.Selection, error) {
	native := make([]selection.Child, len(children))
	for i, child := range children {
		native[i] = nativeSelectionChild(child)
	}
	result, err := selection.MaterializeAdjustments(native, nativeOfferingCatalog(openByID), selectionMode, selection.Grandfathered{}, false)
	for i := range native {
		applyNativeSelection(&children[i], native[i])
	}
	return result, err
}

// materializeChildOfferingSelection validates one child's picks against its
// own catalog, the way a change request validates each child.
func materializeChildOfferingSelection(index int, child *SubmitChild, catalog map[int64]*enrollmentModels.CareOffering, selectionMode string) ([]selection.Selection, error) {
	native := nativeSelectionChild(*child)
	selections, err := selection.MaterializeChild(index, &native, nativeOfferingCatalog(catalog), selectionMode)
	if err != nil {
		return nil, err
	}
	applyNativeSelection(child, native)
	return selections, nil
}

func childrenWithMaterializedOfferingSelections(children []SubmitChild, materialized [][]selection.Selection) []SubmitChild {
	withMaterialized := make([]SubmitChild, len(children))
	for i, child := range children {
		withMaterialized[i] = child
		if i >= len(materialized) {
			continue
		}
		ids := make([]int64, 0, len(materialized[i]))
		for _, pick := range materialized[i] {
			ids = append(ids, pick.OfferingID)
		}
		withMaterialized[i].OfferingIDs = ids
	}
	return withMaterialized
}

// preservedOfferingSelections copies the stored links onto replacement child
// rows while the catalog is disabled. The parent cannot see or change them,
// but re-enabling the setting restores them intact.
func preservedOfferingSelections(existingChildren []*RequestChild, incoming []SubmitChild, links []*enrollment.RequestChildOfferingRecord) [][]selection.Selection {
	byChild := make(map[int64][]selection.Selection, len(existingChildren))
	for _, link := range links {
		byChild[link.RequestChildID] = append(byChild[link.RequestChildID], selection.Selection{
			OfferingID:            link.CareOfferingID,
			SelectedDays:          copyDays(link.SelectedDays),
			ManualSelectedDays:    copyDays(link.ManualSelectedDays),
			AutomaticSelectedDays: copyDays(link.AutomaticSelectedDays),
		})
	}
	result := make([][]selection.Selection, len(incoming))
	for i, child := range matchExistingChildrenBySubmittedIdentity(existingChildren, incoming) {
		if child != nil {
			result[i] = byChild[child.ID]
		}
	}
	return result
}

func copyDays(days []string) []string {
	if len(days) == 0 {
		return nil
	}
	out := make([]string, len(days))
	copy(out, days)
	return out
}

// fullWindowClaims converts submit-time selections into capacity claims
// spanning the whole phase window.
func fullWindowClaims(children []SubmitChild) [][]enrollment.OfferingClaim {
	claims := make([][]enrollment.OfferingClaim, len(children))
	for i, child := range children {
		childClaims := make([]enrollment.OfferingClaim, 0, len(child.OfferingIDs))
		for _, offeringID := range child.OfferingIDs {
			childClaims = append(childClaims, enrollment.OfferingClaim{OfferingID: offeringID})
		}
		claims[i] = childClaims
	}
	return claims
}

// applyCapacityOverflow runs the children's claims through the owner's
// capacity gate. It returns the status a child must take instead of
// submitted, sparse by child index. preservedClaims keeps the slots an edit
// already held; replacedRequestChildIDs excludes the claims an approval
// replaces.
func (s *Intake) applyCapacityOverflow(ctx context.Context, phase *enrollment.Phase, children []SubmitChild, preservedClaims map[int64]int, replacedRequestChildIDs []int64) (map[int]string, error) {
	if s.deps.Capacity == nil {
		return nil, errors.New("care offering capacity gate is not configured")
	}
	return s.deps.Capacity.ApplyCapacityOverflow(ctx, enrollment.CapacityCheck{
		Phase: phase, Claims: fullWindowClaims(children),
		PreservedClaims: preservedClaims, ReplacedRequestChildIDs: replacedRequestChildIDs,
	})
}

// recordOfferingSubmission records a child's submitted offering choices in
// Enrollment and the effective bookings of the phase window in Care Plan.
func (s *Intake) recordOfferingSubmission(ctx context.Context, childID int64, selections []selection.Selection, from, through enrollment.Date) error {
	if len(selections) == 0 {
		return nil
	}
	if s.deps.Bookings == nil {
		return errors.New("request submission requires Care Plan booking commands")
	}
	start, until := calendar.Date(from), calendar.Date(through).AddDays(1)
	choices := make([]enrollment.SubmittedOfferingChoice, 0, len(selections))
	bookings := make([]enrollment.CareBookingInput, 0, len(selections))
	for _, pick := range selections {
		manual := pick.ManualSelectedDays
		if len(manual) == 0 && len(pick.AutomaticSelectedDays) == 0 {
			manual = pick.SelectedDays
		}
		choices = append(choices, enrollment.SubmittedOfferingChoice{CareOfferingID: pick.OfferingID, SelectedDays: manual})
		bookings = append(bookings, enrollment.CareBookingInput{
			CareOfferingID: pick.OfferingID, ManualSelectedDays: manual, AutomaticSelectedDays: pick.AutomaticSelectedDays,
			ValidFrom: &start, ValidUntil: &until,
		})
	}
	if err := s.deps.Children.RecordSubmittedOfferingChoices(ctx, childID, choices); err != nil {
		return fmt.Errorf("record submitted offering choices: %w", err)
	}
	if err := s.deps.Bookings.RecordCareBookings(ctx, childID, bookings); err != nil {
		return fmt.Errorf("record effective care bookings: %w", err)
	}
	return nil
}

// selectedOfferingNames resolves a child's picks to the lower-cased offering
// names the care-offering conditions of the form match on.
func selectedOfferingNames(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) map[string]bool {
	names := make(map[string]bool, len(child.OfferingIDs))
	for _, id := range child.OfferingIDs {
		if o, ok := openByID[id]; ok {
			names[lowerTrim(o.Name)] = true
		}
	}
	return names
}

// offeringsByID indexes offering rows by id.
func offeringsByID(offerings []*enrollmentModels.CareOffering) map[int64]*enrollmentModels.CareOffering {
	byID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		byID[offering.ID] = offering
	}
	return byID
}
