package test

import "testing"

func TestHolidayContributionTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range holidayContributionTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedHolidayContributionTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed holiday contribution boundaries were not removed exactly: %#v", remaining)
	}
	for site := range holidayContributionTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedHolidayContributionTransactions(current)) != len(current) {
		t.Fatal("a reworded holiday contribution boundary escaped the shrink-only check")
	}
}
