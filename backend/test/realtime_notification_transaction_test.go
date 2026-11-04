package test

import "testing"

func TestRealtimeNotificationTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range realtimeNotificationTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedRealtimeNotificationTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed notification boundaries were not removed exactly: %#v", remaining)
	}
	if len(current) != 3 {
		t.Fatal("filter mutated its input")
	}
	for site := range realtimeNotificationTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedRealtimeNotificationTransactions(current)) != len(current) {
		t.Fatal("a reworded notification boundary escaped the shrink-only check")
	}
}
