package enrollment

import (
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
)

func nativeOfferingCatalog(catalog map[int64]*enrollmentModels.CareOffering) map[int64]*selection.Offering {
	result := make(map[int64]*selection.Offering, len(catalog))
	for id, offering := range catalog {
		if offering == nil {
			continue
		}
		native := &selection.Offering{ID: offering.ID, SortOrder: offering.SortOrder, DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays, CountsAsCare: offering.CountsAsCare, IncludesLunch: offering.IncludesLunch, IsRequired: offering.IsRequired, SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule, AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, AutoAddGradeLevels: offering.AutoAddGradeLevels}
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
