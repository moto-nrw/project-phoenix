package enrollment

import (
	"fmt"
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
	// Build group → rule from the catalog. Offerings in the same group
	// share a rule; the editor keeps them consistent, so the last
	// non-optional rule seen for a group wins deterministically.
	groupRule := map[string]string{}
	for _, o := range openByID {
		group := strings.TrimSpace(o.SelectionGroup)
		if group == "" || o.SelectionRule == "" || o.SelectionRule == enrollmentModels.SelectionRuleOptional {
			continue
		}
		groupRule[group] = o.SelectionRule
	}
	if len(groupRule) == 0 {
		return nil
	}

	for idx := range children {
		counts := offeringGroupCounts(children[idx], openByID)
		for group, rule := range groupRule {
			if err := checkGroupRule(idx, group, rule, counts[group]); err != nil {
				return err
			}
		}
	}
	return nil
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
