package architecture

// reviewedSeedMealHistoryRule pins ADR 0189's CLI wiring to the single
// Meal Plan composition package, without granting access to its persistence.
func reviewedSeedMealHistoryRule(rule Rule) bool {
	return rule.SourceOwner == "dev-tools" && rule.SourceOwnerKind == "" && rule.SourceRole == "compose" && rule.SourcePackage == "" &&
		rule.TargetOwner == "meal-plan" && rule.TargetOwnerKind == "" && rule.TargetRole == "compose" &&
		rule.TargetPackage == "modules/mealplan/compose" && rule.TargetClass == "" && !rule.SameOwner &&
		len(rule.Scopes) == 1 && rule.Scopes[0] == string(ScopeProduction)
}
