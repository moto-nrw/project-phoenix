package test

import "testing"

func TestTransactionFrameBundleExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range transactionFrameBundleSites {
		current[site] = reason
	}
	remaining := withoutReviewedTransactionFrameBundles(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed frame bundle boundaries were not removed exactly: %#v", remaining)
	}
	for site := range transactionFrameBundleSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedTransactionFrameBundles(current)) != len(current) {
		t.Fatal("a reworded frame bundle boundary escaped the shrink-only check")
	}
}
