package test

import (
	"strings"
	"testing"
)

func TestSeedMealTransactionOnlyRelocatesExistingBoundary(t *testing.T) {
	t.Parallel()
	const from = "modules/mealplan/compose/new.go:New:tenant.NewTransactionRunner().RunInTx#1"
	const to = "modules/mealplan/compose/new.go:transaction.Run:tenant.NewTransactionRunner().RunInTx#1"
	previous := map[string]string{from: "original reason"}
	current := map[string]string{to: previous[from]}
	if got := originalSeedMealTransactionKey(to, current, previous); got != from {
		t.Fatalf("relocation: %q", got)
	}
	if got := originalSeedMealTransactionKey(to, current, map[string]string{}); got != "" {
		t.Fatal("requires an existing boundary")
	}
	current[from] = previous[from]
	if got := originalSeedMealTransactionKey(to, current, previous); got != "" {
		t.Fatal("cannot copy a boundary")
	}
	delete(current, from)
	current[to] = "new reason"
	if got := originalSeedMealTransactionKey(to, current, previous); got != "" {
		t.Fatal("cannot change its reason")
	}
	for _, key := range []string{strings.Replace(to, "mealplan", "other", 1), strings.Replace(to, "RunInTx", "RunInTxWithRetry", 1), strings.Replace(to, "#1", "#2", 1)} {
		if got := originalSeedMealTransactionKey(key, map[string]string{key: previous[from]}, previous); got != "" {
			t.Fatalf("admitted another site: %s", key)
		}
	}
}
