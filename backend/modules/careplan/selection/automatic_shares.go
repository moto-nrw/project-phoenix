package selection

import "slices"

// AutomaticShare distinguishes optional rule contributions from manual days
// and required lunch. Labels remain the responsibility of the caller.
type AutomaticShare struct {
	AutomaticDays    []string
	RuleDays         []string
	DaysWithoutRules []string
	TriggerIDs       []int64
}

func AutomaticShares(materialized []Selection, catalog map[int64]*Offering) map[int64]AutomaticShare {
	selections := make(map[int64]*Selection, len(materialized))
	for i := range materialized {
		selections[materialized[i].OfferingID] = &materialized[i]
	}
	shares := make(map[int64]AutomaticShare)
	for id, selected := range selections {
		if len(selected.AutomaticSelectedDays) == 0 {
			continue
		}
		share := AutomaticShare{AutomaticDays: slices.Clone(selected.AutomaticSelectedDays)}
		if target := catalog[id]; target != nil {
			addRuleShare(&share, target, selected, selections, catalog)
		}
		shares[id] = share
	}
	return shares
}

// addRuleShare records the automatic days only the auto-add rules contribute,
// beyond manual days and required lunch, and the triggers that produce them.
func addRuleShare(share *AutomaticShare, target *Offering, selected *Selection, selections map[int64]*Selection, catalog map[int64]*Offering) {
	withoutRules := unionDaysInOfferingOrder(target.AvailableDays, selected.ManualSelectedDays, autoLunchDaysForTarget(target, selections, catalog))
	ruleDays := autoDaysForTarget(target, target.AutoAddTriggerOfferingIDs, selections, catalog)
	ruleDays = slices.DeleteFunc(slices.Clone(ruleDays), func(day string) bool { return slices.Contains(withoutRules, day) })
	if len(ruleDays) == 0 {
		return
	}
	share.RuleDays = ruleDays
	share.DaysWithoutRules = withoutRules
	for _, triggerID := range target.AutoAddTriggerOfferingIDs {
		triggerDays := autoDaysForTarget(target, []int64{triggerID}, selections, catalog)
		if slices.ContainsFunc(triggerDays, func(day string) bool { return slices.Contains(ruleDays, day) }) {
			share.TriggerIDs = append(share.TriggerIDs, triggerID)
		}
	}
}
