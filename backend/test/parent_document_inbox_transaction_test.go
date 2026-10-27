package test

import "testing"

func TestParentDocumentInboxTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range parentDocumentInboxTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedParentDocumentInboxTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed document inbox boundaries were not removed exactly: %#v", remaining)
	}
	for site := range parentDocumentInboxTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedParentDocumentInboxTransactions(current)) != len(current) {
		t.Fatal("a reworded document inbox boundary escaped the shrink-only check")
	}
}
