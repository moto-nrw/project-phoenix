package test

// ADR 0190 extracts the existing Meal Plan runner binding into one shared
// adapter. Both constructors use it; the register keeps exactly one boundary
// and its original reason. No additional transaction site is admitted.
func originalSeedMealTransactionKey(key string, current, previous map[string]string) string {
	const from = "modules/mealplan/compose/new.go:New:tenant.NewTransactionRunner().RunInTx#1"
	const to = "modules/mealplan/compose/new.go:transaction.Run:tenant.NewTransactionRunner().RunInTx#1"
	if key != to || previous[from] == "" || current[to] != previous[from] || current[from] != "" {
		return ""
	}
	return from
}
