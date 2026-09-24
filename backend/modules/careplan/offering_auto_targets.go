package careplan

import (
	"fmt"
	"slices"
)

// ExcludedAutoTargetOverrides accepts an exclusion only for an offering that
// the unexcluded materialization marks as a Mitbuchungs-Regel target with
// rule-derived days (#2370). Anything else — a parent-chosen offering, a pure
// required-lunch derivation, an unknown id — cannot be overridden away and
// fails with ErrOfferingChangeInvalid. base is the materialization without
// exclusions; the result names each accepted target once, in the order given.
func ExcludedAutoTargetOverrides(
	excludedIDs []int64,
	base []OfferingSelection,
	offerings map[int64]*CareOffering,
) ([]OfferingOverride, error) {
	if len(excludedIDs) == 0 {
		return nil, nil
	}
	baseByID := selectionPointers(base)
	overridden := make([]OfferingOverride, 0, len(excludedIDs))
	seen := make(map[int64]bool, len(excludedIDs))
	for _, id := range excludedIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		selected, ok := baseByID[id]
		offering := offerings[id]
		if !ok || offering == nil || len(ruleContributionForTarget(offering, selected, baseByID, offerings)) == 0 {
			return nil, fmt.Errorf(
				"%w: offering %d is not added by a co-booking rule and cannot be excluded", ErrOfferingChangeInvalid, id,
			)
		}
		overridden = append(overridden, OfferingOverride{OfferingID: id, Name: offering.Name})
	}
	return overridden, nil
}

func selectionPointers(selections []OfferingSelection) map[int64]*OfferingSelection {
	byID := make(map[int64]*OfferingSelection, len(selections))
	for i := range selections {
		byID[selections[i].OfferingID] = &selections[i]
	}
	return byID
}

// ruleContributionForTarget is the part of a target's days only its
// Mitbuchungs-Regel adds: the trigger-derived days minus the manual days and
// the required-lunch derivation.
func ruleContributionForTarget(
	target *CareOffering,
	selected *OfferingSelection,
	selections map[int64]*OfferingSelection,
	offerings map[int64]*CareOffering,
) []string {
	if target == nil || selected == nil {
		return nil
	}
	ruleDays := autoDaysForTarget(target, selections, offerings)
	nonRuleDays := unionDaysInOfferingOrder(
		target.AvailableDays,
		selected.ManualSelectedDays,
		autoLunchDaysForTarget(target, selections, offerings),
	)
	return slices.DeleteFunc(slices.Clone(ruleDays), func(day string) bool {
		return slices.Contains(nonRuleDays, day)
	})
}

// autoDaysForTarget returns the target's available days its selected triggers
// book, in the target's day order. A fixed trigger books all its days.
func autoDaysForTarget(target *CareOffering, selections map[int64]*OfferingSelection, offerings map[int64]*CareOffering) []string {
	targetDays := dayLookup(target.AvailableDays)
	selected := make(map[string]bool, len(target.AvailableDays))
	for _, triggerID := range target.AutoAddTriggerOfferingIDs {
		triggerSelection := selections[triggerID]
		trigger := offerings[triggerID]
		if triggerSelection == nil || trigger == nil {
			continue
		}
		markBookedDays(selected, targetDays, trigger, triggerSelection)
	}
	return daysFromSetInOrder(target.AvailableDays, selected)
}

// autoLunchDaysForTarget returns the days a required parent-choice lunch
// offering follows the child's other care bookings on.
func autoLunchDaysForTarget(target *CareOffering, selections map[int64]*OfferingSelection, offerings map[int64]*CareOffering) []string {
	if !target.IsRequired || !target.IncludesLunch || target.DaysOfWeekMode != "parent_choice" {
		return nil
	}
	targetDays := dayLookup(target.AvailableDays)
	selected := make(map[string]bool, len(target.AvailableDays))
	for offeringID, selection := range selections {
		if offeringID == target.ID || selection == nil {
			continue
		}
		offering := offerings[offeringID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
		markBookedDays(selected, targetDays, offering, selection)
	}
	return daysFromSetInOrder(target.AvailableDays, selected)
}

func dayLookup(days []string) map[string]bool {
	lookup := make(map[string]bool, len(days))
	for _, day := range days {
		lookup[day] = true
	}
	return lookup
}

// markBookedDays marks the days the offering's selection books that the
// target offers.
func markBookedDays(selected, targetDays map[string]bool, offering *CareOffering, selection *OfferingSelection) {
	days := selection.SelectedDays
	if offering.DaysOfWeekMode == "fixed" {
		days = offering.AvailableDays
	}
	for _, day := range days {
		if targetDays[day] {
			selected[day] = true
		}
	}
}

func unionDaysInOfferingOrder(available []string, groups ...[]string) []string {
	seen := make(map[string]bool, len(available))
	for _, group := range groups {
		for _, day := range group {
			seen[day] = true
		}
	}
	return daysFromSetInOrder(available, seen)
}

func daysFromSetInOrder(order []string, selected map[string]bool) []string {
	out := make([]string, 0, len(selected))
	for _, day := range order {
		if selected[day] {
			out = append(out, day)
		}
	}
	return out
}
