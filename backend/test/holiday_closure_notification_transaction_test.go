package test

import "testing"

func TestHolidayClosureNotificationTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range holidayClosureNotificationTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedHolidayClosureNotificationTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed holiday closure notification boundaries were not removed exactly: %#v", remaining)
	}
	for site := range holidayClosureNotificationTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedHolidayClosureNotificationTransactions(current)) != len(current) {
		t.Fatal("a reworded holiday closure notification boundary escaped the shrink-only check")
	}
}
