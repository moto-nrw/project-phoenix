package selection

// Values of the offering and phase settings the engine reads, mirrored from
// models/enrollment, which this package does not import.
const (
	daysOfWeekModeFixed        = "fixed"
	daysOfWeekModeParentChoice = "parent_choice"

	selectionRuleOptional   = "optional"
	selectionRuleExactlyOne = "exactly_one"
	selectionRuleAtLeastOne = "at_least_one"
	selectionRuleAtMostOne  = "at_most_one"

	selectionModeOptional   = "optional"
	selectionModeExactlyOne = "exactly_one"
	selectionModeAtLeastOne = "at_least_one"

	availabilityOperatorIn    = "in"
	availabilityOperatorNotIn = "not_in"
)

// validateOfferingGroupRules is the ordinary group-rule check: a required
// group may not stay empty.
func validateOfferingGroupRules(children []Child, openByID map[int64]*Offering) error {
	return validateOfferingGroupRulesWithMissingRequiredAllowed(children, openByID, false)
}

// validateOfferingSelections cross-checks every picked offering against the
// open catalog, child by child.
func validateOfferingSelections(children []Child, openByID map[int64]*Offering) error {
	for _, child := range children {
		if err := validateOfferingSelectionsForChild(child, openByID, openByID); err != nil {
			return err
		}
	}
	return nil
}

// parentChoiceMissingDays is the refusal materializeOfferingSelections, the
// caller of resolveManualSelectedDays, returns for a parent_choice offering
// picked without a day.
func parentChoiceMissingDays() error {
	offering := parentChoiceOffering("mon", "tue", "wed")
	offering.ID = 1
	_, err := materializeOfferingSelections(Child{OfferingIDs: []int64{offering.ID}}, map[int64]*Offering{offering.ID: offering})
	return err
}
