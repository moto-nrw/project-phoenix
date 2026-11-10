package test

import "testing"

func TestVisitRetentionTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{visitRetentionTransactionSite: visitRetentionTransactionReason, "unrelated": "unchanged"}
	remaining := withoutReviewedVisitRetentionTransaction(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != 2 {
		t.Fatal("reviewed boundary changed another register entry")
	}
	current[visitRetentionTransactionSite] = "different reason"
	if len(withoutReviewedVisitRetentionTransaction(current)) != 2 {
		t.Fatal("changed boundary reason was admitted")
	}
}
