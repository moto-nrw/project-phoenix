package enrollment

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// validateOfferingGroupRules enforces per-group selection rules for every
// child against the open offering catalog. Offerings sharing a non-empty
// selection_group are constrained together by the group's rule:
//
//   - exactly_one  → the child must pick exactly one of the group
//   - at_least_one → the child must pick one or more
//   - at_most_one  → the child may pick zero or one, never several
//   - optional     → no constraint (also the ungrouped default)
//
// Defense-in-depth: the parent form enforces the same rules (and prevents
// over-selection interactively), but a stale or scripted submit must not
// slip past. Mirrors groupOfferings/validation in enrollment-form.tsx.
func validateOfferingGroupRules(children []SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) error {
	return validateOfferingGroupRulesWithMissingRequiredAllowed(children, openByID, false)
}

// validateOfferingGroupRulesWithMissingRequiredAllowed keeps the upper bounds
// and configuration checks intact while allowing a confirmed complete
// withdrawal to leave a required group empty. In particular, at_most_one and
// the "at most" half of exactly_one remain enforced for non-care offerings
// that stay booked.
func validateOfferingGroupRulesWithMissingRequiredAllowed(
	children []SubmitChild,
	openByID map[int64]*enrollmentModels.CareOffering,
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
		if group == "" || o.SelectionRule == "" || o.SelectionRule == enrollmentModels.SelectionRuleOptional {
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
	if allowMissingRequired && count == 0 && (rule == enrollmentModels.SelectionRuleExactlyOne || rule == enrollmentModels.SelectionRuleAtLeastOne) {
		return nil
	}
	return checkGroupRule(childIdx, group, rule, count)
}

// offeringGroupCounts counts how many offerings the child selected in
// each non-empty selection_group.
func offeringGroupCounts(child SubmitChild, openByID map[int64]*enrollmentModels.CareOffering) map[string]int {
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
	case enrollmentModels.SelectionRuleExactlyOne:
		if count != 1 {
			return fmt.Errorf("%w: child %d group %q requires exactly one selection", ErrCareOfferingRule, childIdx, group)
		}
	case enrollmentModels.SelectionRuleAtLeastOne:
		if count < 1 {
			return fmt.Errorf("%w: child %d group %q requires at least one selection", ErrCareOfferingRule, childIdx, group)
		}
	case enrollmentModels.SelectionRuleAtMostOne:
		if count > 1 {
			return fmt.Errorf("%w: child %d group %q allows at most one selection", ErrCareOfferingRule, childIdx, group)
		}
	}
	return nil
}
