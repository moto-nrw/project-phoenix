// Package selection calculates and validates the care offering selections of
// a booking: the catalog rules, automatic additions and lunch days. Care Plan
// owns the care offerings; Enrollment, which sits above Care Plan (ADR 0058,
// #3417), applies the rules when it materializes an application.
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
		selections, err := materializeAdjustmentChild(i, &children[i], openByID, selectionMode, grandfathered, allowCompleteWithdrawal)
		if err != nil {
			return nil, err
		}
		out[i] = selections
	}
	return out, nil
}

// materializeAdjustmentChild validates and materializes one child of an
// adjustment. The child's selection is replaced by its materialization before
// the group, required and selection-mode rules run, so a later rule failure
// leaves the child in the normalized state.
func materializeAdjustmentChild(
	index int,
	child *Child,
	openByID map[int64]*Offering,
	selectionMode string,
	grandfathered Grandfathered,
	allowCompleteWithdrawal bool,
) ([]Selection, error) {
	availableByID, err := availableCareOfferingsForGrade(openByID, child.TargetGradeLevel)
	if err != nil {
		return nil, fmt.Errorf("child %d: %w", index, err)
	}
	for id := range grandfatheredStillSelected(*child, grandfathered.Manual) {
		if offering, ok := openByID[id]; ok {
			availableByID[id] = offering
		}
	}
	if err := validateOfferingSelectionsForChild(*child, openByID, availableByID); err != nil {
		return nil, fmt.Errorf("child %d: %w", index, err)
	}
	materializeByID := withGrandfatheredAutomatic(availableByID, openByID, grandfathered.Automatic)
	manualChild := cloneChildrenOfferingSelections([]Child{*child})[0]
	selections, err := materializeOfferingSelections(*child, materializeByID)
	if err != nil {
		return nil, fmt.Errorf("child %d: %w", index, err)
	}
	child.OfferingIDs, child.OfferingDays = selectionPayload(selections, materializeByID)
	completeWithdrawal := allowCompleteWithdrawal && !materializedSelectionsHaveCareDays(selections, materializeByID)
	if err := validateOfferingGroupRulesWithMissingRequiredAllowed(
		[]Child{*child}, availableByID, completeWithdrawal,
	); err != nil {
		return nil, withChildIndex(err, index)
	}
	// Grandfathered automatic offerings are part of the materialized booking
	// and must take part in per-day conflicts, while they remain unavailable
	// for the ordinary group rules above.
	if err := CheckDayRules(*child, materializeByID); err != nil {
		return nil, withChildIndex(err, index)
	}
	choosable := len(openByID) == 0 || hasChoosableCareOffering(availableByID)
	if err := validateAdjustmentRequirements(*child, manualChild, availableByID, selectionMode, choosable, completeWithdrawal); err != nil {
		return nil, err
	}
	return selections, nil
}

// withGrandfatheredAutomatic widens the catalog for auto-add: a holding's
// automatic days can be re-derived without the offering becoming selectable,
// required, or choosable. Without automatic holdings the catalog is returned
// unchanged.
func withGrandfatheredAutomatic(availableByID, openByID map[int64]*Offering, automatic map[int64]bool) map[int64]*Offering {
	if len(automatic) == 0 {
		return availableByID
	}
	materializeByID := make(map[int64]*Offering, len(availableByID)+len(automatic))
	for id, offering := range availableByID {
		materializeByID[id] = offering
	}
	for id := range automatic {
		if offering, ok := openByID[id]; ok {
			materializeByID[id] = offering
		}
	}
	return materializeByID
}

// validateAdjustmentRequirements checks the required offerings and the
// selection mode. A complete withdrawal skips the required offerings and
// tolerates a missing manual choice.
func validateAdjustmentRequirements(
	child, manualChild Child,
	availableByID map[int64]*Offering,
	selectionMode string,
	choosable, completeWithdrawal bool,
) error {
	if completeWithdrawal {
		if !choosable {
			return nil
		}
		return validateCareOfferingSelectionModeAllowingMissing([]Child{manualChild}, availableByID, selectionMode)
	}
	if err := validateRequiredOfferings([]Child{child}, availableByID); err != nil {
		return err
	}
	if !choosable {
		return nil
	}
	return validateCareOfferingSelectionMode([]Child{manualChild}, availableByID, selectionMode)
}

// MaterializeChild materializes one child against the child's own catalog,
// the way a change request validates each child separately: an unknown
// offering is closed before any availability rule is read, and index names
// the child in the errors that locate it. On success the child's selection is
// replaced by its materialization.
func MaterializeChild(index int, child *Child, catalog map[int64]*Offering, selectionMode string) ([]Selection, error) {
	for _, id := range child.OfferingIDs {
		if _, ok := catalog[id]; !ok {
			return nil, fmt.Errorf("child %d: %w", index, ErrCareOfferingClosed)
		}
	}
	available, err := availableCareOfferingsForGrade(catalog, child.TargetGradeLevel)
	if err != nil {
		return nil, fmt.Errorf("child %d: %w", index, err)
	}
	if err := validateOfferingSelectionsForChild(*child, catalog, available); err != nil {
		return nil, fmt.Errorf("child %d: %w", index, err)
	}
	manualChild := cloneChildrenOfferingSelections([]Child{*child})[0]
	selections, err := materializeOfferingSelections(*child, available)
	if err != nil {
		return nil, fmt.Errorf("child %d: %w", index, err)
	}
	child.OfferingIDs, child.OfferingDays = selectionPayload(selections, available)
	if err := validateOfferingGroupRulesWithMissingRequiredAllowed([]Child{*child}, available, false); err != nil {
		return nil, withChildIndex(err, index)
	}
	if err := validateRequiredOfferings([]Child{*child}, available); err != nil {
		return nil, err
	}
	if len(catalog) == 0 || hasChoosableCareOffering(available) {
		if err := validateCareOfferingSelectionMode([]Child{manualChild}, available, selectionMode); err != nil {
			return nil, err
		}
	}
	return selections, nil
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
	selectionByID, err := manualOfferingSelections(child, openByID)
	if err != nil {
		return nil, err
	}
	targets := sortedCareOfferings(openByID)
	for _, target := range targets {
		if careOfferingCanAutoAddDays(target) && autoAddAppliesToGrade(child.TargetGradeLevel, target.AutoAddGradeLevels) && target.DaysOfWeekMode != "parent_choice" {
			return nil, fmt.Errorf("offering %d cannot be automatically added because it does not allow day selection", target.ID)
		}
	}
	for changed := true; changed; {
		changed = applyAutoAdditions(child, targets, selectionByID, openByID)
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
	return sortedSelections(selectionByID, openByID), nil
}

// manualOfferingSelections resolves the child's manual picks per selected
// offering, before any automatic addition.
func manualOfferingSelections(child Child, openByID map[int64]*Offering) (map[int64]*Selection, error) {
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
	return selectionByID, nil
}

// applyAutoAdditions runs one pass of the auto-add rules over every target in
// catalog order and reports whether any selection changed. Callers repeat the
// pass until it is stable, because one addition can trigger another.
func applyAutoAdditions(child Child, targets []*Offering, selectionByID map[int64]*Selection, openByID map[int64]*Offering) bool {
	changed := false
	for _, target := range targets {
		if !careOfferingCanAutoAddDays(target) || !autoAddAppliesToGrade(child.TargetGradeLevel, target.AutoAddGradeLevels) {
			continue
		}
		if applyAutoAddTarget(child, target, selectionByID, openByID) {
			changed = true
		}
	}
	return changed
}

func applyAutoAddTarget(child Child, target *Offering, selectionByID map[int64]*Selection, openByID map[int64]*Offering) bool {
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
		return false
	}
	changed := false
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
	return changed
}

func sortedSelections(selectionByID map[int64]*Selection, openByID map[int64]*Offering) []Selection {
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
	return out
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
		for _, day := range lunchSourceDays(target, offeringID, selection, openByID) {
			if targetDays[day] {
				selected[day] = true
			}
		}
	}
	return daysFromSetInOrder(target.AvailableDays, selected)
}

// lunchSourceDays returns the care days another selected care offering
// contributes to a required lunch target, or nil when it contributes none.
func lunchSourceDays(target *Offering, offeringID int64, selection *Selection, openByID map[int64]*Offering) []string {
	if offeringID == target.ID || selection == nil {
		return nil
	}
	offering := openByID[offeringID]
	if offering == nil || !offering.CountsAsCare {
		return nil
	}
	if offering.DaysOfWeekMode == "fixed" {
		return offering.AvailableDays
	}
	return selection.SelectedDays
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
		if err := validateChildSelectionMode(i, child, openByID, mode, allowMissing); err != nil {
			return err
		}
	}
	return nil
}

func validateChildSelectionMode(index int, child Child, openByID map[int64]*Offering, mode string, allowMissing bool) error {
	selected := make(map[int64]bool, len(child.OfferingIDs))
	for _, id := range child.OfferingIDs {
		selected[id] = true
	}
	for _, dayPick := range child.OfferingDays {
		if !selected[dayPick.OfferingID] {
			return fmt.Errorf("%w: child %d offering %d has days but is not selected", ErrInvalidSubmission, index, dayPick.OfferingID)
		}
	}
	choosableCount := choosableSelectionCount(child, openByID)
	switch mode {
	case "at_least_one":
		if choosableCount == 0 && !allowMissing {
			return fmt.Errorf("%w: child %d", ErrCareOfferingMissing, index)
		}
	case "exactly_one":
		if choosableCount > 1 || (choosableCount == 0 && !allowMissing) {
			return fmt.Errorf("%w: child %d", ErrCareOfferingExactlyOneRequired, index)
		}
	}
	return nil
}

// choosableSelectionCount counts only the choosable (non-required) selected
// offerings. An offering absent from openByID is rejected earlier by
// validateOfferingSelections, so unknown ids count as choosable.
func choosableSelectionCount(child Child, openByID map[int64]*Offering) int {
	count := 0
	for _, id := range child.OfferingIDs {
		if o, ok := openByID[id]; !ok || !o.IsRequired {
			count++
		}
	}
	return count
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
		if err := checkChildGroupRules(idx, children[idx], groups, groupRule, openByID, allowMissingRequired); err != nil {
			return err
		}
	}
	return nil
}

// checkChildGroupRules applies every group's rule to one child: the per-day
// rule reads the child's days, the others count the group's offerings.
func checkChildGroupRules(
	idx int,
	child Child,
	groups []string,
	groupRule map[string]string,
	openByID map[int64]*Offering,
	allowMissingRequired bool,
) error {
	counts := offeringGroupCounts(child, openByID)
	for _, group := range groups {
		if groupRule[group] == SelectionRuleAtMostOnePerDay {
			if err := checkDayRule(idx, group, child, openByID); err != nil {
				return err
			}
			continue
		}
		if err := checkGroupRuleAllowingMissingRequired(idx, group, groupRule[group], counts[group], allowMissingRequired); err != nil {
			return err
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
