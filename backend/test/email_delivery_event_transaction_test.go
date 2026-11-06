package test

import "testing"

func TestReviewedEmailDeliveryEventTransactionsAreExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"other": "kept"}
	for site, reason := range emailDeliveryEventTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedEmailDeliveryEventTransactions(current)
	if len(remaining) != 1 || remaining["other"] != "kept" {
		t.Fatalf("reviewed sites must leave only other entries: %v", remaining)
	}
	for site := range emailDeliveryEventTransactionSites {
		changed := map[string]string{site: "another reason"}
		if len(withoutReviewedEmailDeliveryEventTransactions(changed)) != 1 {
			t.Fatalf("a changed reason must stay subject to the register: %s", site)
		}
	}
}
