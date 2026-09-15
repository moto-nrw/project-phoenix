// Package selection calculates and validates enrollment booking selections.
package selection

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
)

// MaterializeAdjustments replaces each child's submitted selection with its
// validated materialization, including automatic shares. On failure, earlier
// children retain the same normalized state as the original adjustment flow.
func MaterializeAdjustments(
	children []Child,
	openByID map[int64]*Offering,
	selectionMode string,
	grandfathered Grandfathered,
	allowCompleteWithdrawal bool,
) ([][]Selection, error) {
	out := make([][]Selection, len(children))
	for i := range children {
		availableByID, err := availableCareOfferingsForGrade(openByID, children[i].TargetGradeLevel)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		for id := range grandfatheredStillSelected(children[i], grandfathered.Manual) {
			if offering, ok := openByID[id]; ok {
				availableByID[id] = offering
			}
		}
		if err := validateOfferingSelectionsForChild(children[i], openByID, availableByID); err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		// Auto-add works off a wider catalog than validation does, so a
		// holding's automatic days can be re-derived without the offering
		// becoming selectable, required, or choosable.
		materializeByID := availableByID
		if len(grandfathered.Automatic) > 0 {
			materializeByID = make(map[int64]*Offering, len(availableByID)+len(grandfathered.Automatic))
			for id, offering := range availableByID {
				materializeByID[id] = offering
			}
			for id := range grandfathered.Automatic {
				if offering, ok := openByID[id]; ok {
					materializeByID[id] = offering
				}
			}
		}
		manualChild := cloneChildrenOfferingSelections([]Child{children[i]})[0]
		selections, err := materializeOfferingSelections(children[i], materializeByID)
		if err != nil {
			return nil, fmt.Errorf("child %d: %w", i, err)
		}
		children[i].OfferingIDs, children[i].OfferingDays = selectionPayload(selections, materializeByID)
		completeWithdrawal := allowCompleteWithdrawal && !materializedSelectionsHaveCareDays(selections, materializeByID)
		if err := validateOfferingGroupRulesWithMissingRequiredAllowed(
			[]Child{children[i]}, availableByID, completeWithdrawal,
		); err != nil {
			return nil, err
		}
		if (len(openByID) == 0 || hasChoosableCareOffering(availableByID)) && completeWithdrawal {
			if err := validateCareOfferingSelectionModeAllowingMissing(
				[]Child{manualChild}, availableByID, selectionMode,
			); err != nil {
				return nil, err
			}
		}
		if completeWithdrawal {
			out[i] = selections
			continue
		}
		if err := validateRequiredOfferings([]Child{children[i]}, availableByID); err != nil {
			return nil, err
		}
		if len(openByID) == 0 || hasChoosableCareOffering(availableByID) {
			if err := validateCareOfferingSelectionMode([]Child{manualChild}, availableByID, selectionMode); err != nil {
				return nil, err
			}
		}
		out[i] = selections
	}
	return out, nil
}

func materializedSelectionsHaveCareDays(
	selections []Selection,
	offerings map[int64]*Offering,
) bool {
	for _, selection := range selections {
		offering := offerings[selection.OfferingID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
		hasCareDays := len(selection.SelectedDays) > 0
		if offering.DaysOfWeekMode == "fixed" {
			hasCareDays = len(offering.AvailableDays) > 0
		}
		if hasCareDays {
			return true
		}
	}
	return false
}

func grandfatheredStillSelected(child Child, grandfathered map[int64]bool) map[int64]bool {
	if len(grandfathered) == 0 {
		return nil
	}
	kept := make(map[int64]bool, len(grandfathered))
	for _, id := range child.OfferingIDs {
		if grandfathered[id] {
			kept[id] = true
		}
	}
	for _, row := range child.OfferingDays {
		if grandfathered[row.OfferingID] {
			kept[row.OfferingID] = true
		}
	}
	return kept
}

func availableCareOfferingsForGrade(catalog map[int64]*Offering, grade *int16) (map[int64]*Offering, error) {
	available := make(map[int64]*Offering, len(catalog))
	for id, offering := range catalog {
		matches, err := offering.AvailabilityRule.MatchesGradeLevel(grade)
		if err != nil {
			return nil, fmt.Errorf("offering %d has invalid availability rule: %w", id, err)
		}
		if matches {
			available[id] = offering
		}
	}
	return available, nil
}

func validateOfferingSelectionsForChild(child Child, catalog, available map[int64]*Offering) error {
	check := func(id int64) error {
		if _, ok := catalog[id]; !ok {
			return ErrCareOfferingClosed
		}
		if _, ok := available[id]; !ok {
			return ErrCareOfferingUnavailable
		}
		return nil
	}
	for _, id := range child.OfferingIDs {
		if err := check(id); err != nil {
			return err
		}
	}
	for _, row := range child.OfferingDays {
		if err := check(row.OfferingID); err != nil {
			return err
		}
	}
	return nil
}

func cloneChildrenOfferingSelections(children []Child) []Child {
	out := make([]Child, len(children))
	for i, child := range children {
		out[i] = child
		out[i].OfferingIDs = append([]int64(nil), child.OfferingIDs...)
		if len(child.OfferingDays) > 0 {
			out[i].OfferingDays = make([]DaySelection, len(child.OfferingDays))
			for j, row := range child.OfferingDays {
				out[i].OfferingDays[j] = DaySelection{
					OfferingID:   row.OfferingID,
					SelectedDays: copyDays(row.SelectedDays),
				}
			}
		}
	}
	return out
}

func materializeOfferingSelections(child Child, openByID map[int64]*Offering) ([]Selection, error) {
	daysByOffering := make(map[int64][]string, len(child.OfferingDays))
	for _, row := range child.OfferingDays {
		daysByOffering[row.OfferingID] = row.SelectedDays
	}

	selectionByID := make(map[int64]*Selection, len(child.OfferingIDs))
	for _, offeringID := range child.OfferingIDs {
		offering, ok := openByID[offeringID]
		if !ok {
			return nil, ErrCareOfferingClosed
		}
		manual, err := resolveManualSelectedDays(offering, daysByOffering[offeringID])
		if err != nil {
			return nil, fmt.Errorf("offering %d: %w", offeringID, err)
		}
		selectionByID[offeringID] = &Selection{
			OfferingID:            offeringID,
			SelectedDays:          copyDays(manual),
			ManualSelectedDays:    copyDays(manual),
			AutomaticSelectedDays: nil,
		}
	}

	targets := sortedCareOfferings(openByID)
	for _, target := range targets {
		if careOfferingCanAutoAddDays(target) && autoAddAppliesToGrade(child.TargetGradeLevel, target.AutoAddGradeLevels) && target.DaysOfWeekMode != "parent_choice" {
			return nil, fmt.Errorf("offering %d cannot be automatically added because it does not allow day selection", target.ID)
		}
	}

	changed := true
	for changed {
		changed = false
		for _, target := range targets {
			if !careOfferingCanAutoAddDays(target) || !autoAddAppliesToGrade(child.TargetGradeLevel, target.AutoAddGradeLevels) {
				continue
			}
			var ruleDays []string
			if !child.ExcludedAutoAddTargetIDs[target.ID] {
				ruleDays = autoDaysForTarget(target, target.AutoAddTriggerOfferingIDs, selectionByID, openByID)
			}
			autoDays := unionDaysInOfferingOrder(
				target.AvailableDays,
				ruleDays,
				autoLunchDaysForTarget(target, selectionByID, openByID),
			)
			if len(autoDays) == 0 {
				continue
			}
			selection := selectionByID[target.ID]
			if selection == nil {
				selection = &Selection{OfferingID: target.ID}
				selectionByID[target.ID] = selection
				changed = true
			}
			if !slices.Equal(selection.AutomaticSelectedDays, autoDays) {
				selection.AutomaticSelectedDays = autoDays
				changed = true
			}
			selectedDays := unionDaysInOfferingOrder(target.AvailableDays, selection.ManualSelectedDays, selection.AutomaticSelectedDays)
			if !slices.Equal(selection.SelectedDays, selectedDays) {
				selection.SelectedDays = selectedDays
				changed = true
			}
		}
	}

	for offeringID, selection := range selectionByID {
		offering := openByID[offeringID]
		if offering == nil || offering.DaysOfWeekMode != "parent_choice" {
			continue
		}
		if len(selection.SelectedDays) == 0 {
			return nil, fmt.Errorf("offering %d: %w", offeringID, ErrDaySelectionRequired)
		}
	}

	out := make([]Selection, 0, len(selectionByID))
	for _, selection := range selectionByID {
		out = append(out, *selection)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := openByID[out[i].OfferingID]
		right := openByID[out[j].OfferingID]
		if left == nil || right == nil {
			return out[i].OfferingID < out[j].OfferingID
		}
		if left.SortOrder == right.SortOrder {
			return left.ID < right.ID
		}
		return left.SortOrder < right.SortOrder
	})
	return out, nil
}

func sortedCareOfferings(openByID map[int64]*Offering) []*Offering {
	out := make([]*Offering, 0, len(openByID))
	for _, offering := range openByID {
		if offering != nil {
			out = append(out, offering)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SortOrder == out[j].SortOrder {
			return out[i].ID < out[j].ID
		}
		return out[i].SortOrder < out[j].SortOrder
	})
	return out
}

func selectionPayload(selections []Selection, openByID map[int64]*Offering) ([]int64, []DaySelection) {
	ids := make([]int64, 0, len(selections))
	dayRows := make([]DaySelection, 0, len(selections))
	for _, selection := range selections {
		ids = append(ids, selection.OfferingID)
		if offering := openByID[selection.OfferingID]; offering != nil && offering.DaysOfWeekMode == "parent_choice" {
			dayRows = append(dayRows, DaySelection{
				OfferingID:   selection.OfferingID,
				SelectedDays: copyDays(selection.SelectedDays),
			})
		}
	}
	return ids, dayRows
}

func autoAddAppliesToGrade(grade *int16, levels []int) bool {
	if len(levels) == 0 {
		return true
	}
	if grade == nil {
		return false
	}
	for _, level := range levels {
		if int(*grade) == level {
			return true
		}
	}
	return false
}

func careOfferingCanAutoAddDays(offering *Offering) bool {
	if offering == nil {
		return false
	}
	return len(offering.AutoAddTriggerOfferingIDs) > 0 || isRequiredLunchOffering(offering)
}

func isRequiredLunchOffering(offering *Offering) bool {
	return offering != nil &&
		offering.IsRequired &&
		offering.IncludesLunch &&
		offering.DaysOfWeekMode == "parent_choice"
}

func autoDaysForTarget(
	target *Offering,
	triggerIDs []int64,
	selectionByID map[int64]*Selection,
	openByID map[int64]*Offering,
) []string {
	selected := make(map[string]bool, len(target.AvailableDays))
	targetDays := make(map[string]bool, len(target.AvailableDays))
	for _, day := range target.AvailableDays {
		targetDays[day] = true
	}
	for _, triggerID := range triggerIDs {
		triggerSelection := selectionByID[triggerID]
		if triggerSelection == nil {
			continue
		}
		trigger := openByID[triggerID]
		if trigger == nil {
			continue
		}
		triggerDays := triggerSelection.SelectedDays
		if trigger.DaysOfWeekMode == "fixed" {
			triggerDays = trigger.AvailableDays
		}
		for _, day := range triggerDays {
			if targetDays[day] {
				selected[day] = true
			}
		}
	}
	return daysFromSetInOrder(target.AvailableDays, selected)
}

func autoLunchDaysForTarget(
	target *Offering,
	selectionByID map[int64]*Selection,
	openByID map[int64]*Offering,
) []string {
	if !isRequiredLunchOffering(target) {
		return nil
	}
	selected := make(map[string]bool, len(target.AvailableDays))
	targetDays := make(map[string]bool, len(target.AvailableDays))
	for _, day := range target.AvailableDays {
		targetDays[day] = true
	}
	for offeringID, selection := range selectionByID {
		if offeringID == target.ID || selection == nil {
			continue
		}
		offering := openByID[offeringID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
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
	return daysFromSetInOrder(target.AvailableDays, selected)
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

func copyDays(days []string) []string {
	if len(days) == 0 {
		return nil
	}
	out := make([]string, len(days))
	copy(out, days)
	return out
}

func validateRequiredOfferings(children []Child, openByID map[int64]*Offering) error {
	requiredIDs := make([]int64, 0)
	for id, offering := range openByID {
		if offering.IsRequired {
			requiredIDs = append(requiredIDs, id)
		}
	}
	if len(requiredIDs) == 0 {
		return nil
	}
	for i, child := range children {
		selected := make(map[int64]bool, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			selected[id] = true
		}
		for _, requiredID := range requiredIDs {
			if !selected[requiredID] {
				return fmt.Errorf("%w: child %d offering %d", ErrRequiredCareOfferingMissing, i, requiredID)
			}
		}
	}
	return nil
}

func validateCareOfferingSelectionMode(children []Child, openByID map[int64]*Offering, mode string) error {
	return validateCareOfferingSelectionModeWithMissing(children, openByID, mode, false)
}

func validateCareOfferingSelectionModeAllowingMissing(children []Child, openByID map[int64]*Offering, mode string) error {
	return validateCareOfferingSelectionModeWithMissing(children, openByID, mode, true)
}

func validateCareOfferingSelectionModeWithMissing(children []Child, openByID map[int64]*Offering, mode string, allowMissing bool) error {
	if mode == "" || mode == "optional" {
		return nil
	}
	if mode != "at_least_one" &&
		mode != "exactly_one" {
		return fmt.Errorf("%w: invalid care offering selection mode %q", ErrInvalidSubmission, mode)
	}

	for i, child := range children {
		selected := make(map[int64]bool, len(child.OfferingIDs))
		for _, id := range child.OfferingIDs {
			selected[id] = true
		}
		for _, dayPick := range child.OfferingDays {
			if !selected[dayPick.OfferingID] {
				return fmt.Errorf("%w: child %d offering %d has days but is not selected", ErrInvalidSubmission, i, dayPick.OfferingID)
			}
		}
		// Count only the choosable (non-required) selected offerings. An
		// offering absent from openByID is rejected earlier by
		// validateOfferingSelections, so treat unknown ids as choosable.
		choosableCount := 0
		for _, id := range child.OfferingIDs {
			if o, ok := openByID[id]; !ok || !o.IsRequired {
				choosableCount++
			}
		}
		switch mode {
		case "at_least_one":
			if choosableCount == 0 && !allowMissing {
				return fmt.Errorf("%w: child %d", ErrCareOfferingMissing, i)
			}
		case "exactly_one":
			if choosableCount > 1 || (choosableCount == 0 && !allowMissing) {
				return fmt.Errorf("%w: child %d", ErrCareOfferingExactlyOneRequired, i)
			}
		}
	}
	return nil
}

func hasChoosableCareOffering(catalog map[int64]*Offering) bool {
	for _, offering := range catalog {
		if !offering.IsRequired {
			return true
		}
	}
	return false
}

func resolveManualSelectedDays(offering *Offering, picks []string) ([]string, error) {
	switch offering.DaysOfWeekMode {
	case "fixed":
		if len(picks) > 0 {
			return nil, ErrDaySelectionNotAllowed
		}
		return nil, nil
	case "parent_choice":
		allowed := make(map[string]bool, len(offering.AvailableDays))
		for _, d := range offering.AvailableDays {
			allowed[d] = true
		}
		seen := make(map[string]bool, len(picks))
		dedup := make([]string, 0, len(picks))
		for _, d := range picks {
			if !allowed[d] {
				return nil, fmt.Errorf("%w: day %q is not in the offering's available_days", ErrSelectedDayNotAvailable, d)
			}
			if seen[d] {
				continue
			}
			seen[d] = true
			dedup = append(dedup, d)
		}
		return dedup, nil
	default:
		return nil, fmt.Errorf("offering has unknown days_of_week_mode %q", offering.DaysOfWeekMode)
	}
}

func validateOfferingGroupRulesWithMissingRequiredAllowed(
	children []Child,
	openByID map[int64]*Offering,
	allowMissingRequired bool,
) error {
	// Build group → rule from the catalog. Iterate offerings in a stable
	// order (sorted by id) rather than over the map directly: Go map
	// iteration order is unspecified, so picking the rule from the map would
	// make the chosen rule — and therefore the validation outcome —
	// nondeterministic whenever a group's members disagree, and could
	// diverge from the frontend. Offerings in one group MUST share a single
	// non-optional rule; a group with conflicting rules is an admin
	// misconfiguration we reject rather than silently resolve.
	ids := slices.Sorted(maps.Keys(openByID))

	groupRule := map[string]string{}
	for _, id := range ids {
		o := openByID[id]
		group := strings.TrimSpace(o.SelectionGroup)
		if group == "" || o.SelectionRule == "" || o.SelectionRule == "optional" {
			continue
		}
		if existing, ok := groupRule[group]; ok && existing != o.SelectionRule {
			return fmt.Errorf(
				"%w: selection group %q has conflicting rules %q and %q",
				ErrCareOfferingRule, group, existing, o.SelectionRule,
			)
		}
		groupRule[group] = o.SelectionRule
	}
	if len(groupRule) == 0 {
		return nil
	}

	// Stable group order so the first reported violation is deterministic.
	groups := slices.Sorted(maps.Keys(groupRule))

	for idx := range children {
		counts := offeringGroupCounts(children[idx], openByID)
		for _, group := range groups {
			if err := checkGroupRuleAllowingMissingRequired(idx, group, groupRule[group], counts[group], allowMissingRequired); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkGroupRuleAllowingMissingRequired(childIdx int, group, rule string, count int, allowMissingRequired bool) error {
	if allowMissingRequired && count == 0 && (rule == "exactly_one" || rule == "at_least_one") {
		return nil
	}
	return checkGroupRule(childIdx, group, rule, count)
}

func offeringGroupCounts(child Child, openByID map[int64]*Offering) map[string]int {
	counts := map[string]int{}
	for _, id := range child.OfferingIDs {
		o, ok := openByID[id]
		if !ok {
			continue
		}
		group := strings.TrimSpace(o.SelectionGroup)
		if group == "" {
			continue
		}
		counts[group]++
	}
	return counts
}

func checkGroupRule(childIdx int, group, rule string, count int) error {
	switch rule {
	case "exactly_one":
		if count != 1 {
			return fmt.Errorf("%w: child %d group %q requires exactly one selection", ErrCareOfferingRule, childIdx, group)
		}
	case "at_least_one":
		if count < 1 {
			return fmt.Errorf("%w: child %d group %q requires at least one selection", ErrCareOfferingRule, childIdx, group)
		}
	case "at_most_one":
		if count > 1 {
			return fmt.Errorf("%w: child %d group %q allows at most one selection", ErrCareOfferingRule, childIdx, group)
		}
	}
	return nil
}
