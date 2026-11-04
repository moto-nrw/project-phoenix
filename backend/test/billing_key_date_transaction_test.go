package test

import "testing"

func TestBillingKeyDateTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range billingKeyDateTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedBillingKeyDateTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != len(billingKeyDateTransactionSites)+1 {
		t.Fatal("reviewed boundaries changed another register entry")
	}
	for site := range billingKeyDateTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedBillingKeyDateTransactions(current)) != len(current) {
		t.Fatal("changed boundary reason was admitted")
	}
}
