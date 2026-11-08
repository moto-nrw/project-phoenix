package test

import "testing"

// Only the three seams with exactly their reviewed reasons leave the
// shrink-only transaction register; a reworded reason stays in it.
func TestSessionMintTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range sessionMintTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedSessionMintTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed session seams were not removed exactly: %#v", remaining)
	}
	for site := range sessionMintTransactionSites {
		reworded := map[string]string{"unrelated": "unchanged"}
		for other, reason := range sessionMintTransactionSites {
			reworded[other] = reason
		}
		reworded[site] = "different reason"
		remaining := withoutReviewedSessionMintTransactions(reworded)
		if len(remaining) != 2 || remaining[site] != "different reason" {
			t.Fatalf("a reworded session seam escaped the shrink-only check: %s", site)
		}
	}
}
