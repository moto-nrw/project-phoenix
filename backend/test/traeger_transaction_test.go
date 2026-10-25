package test

import "testing"

func TestTraegerTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range traegerTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedTraegerTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed Träger transaction boundaries were not removed exactly: %#v", remaining)
	}
	for site := range traegerTransactionSites {
		current[site] = "different reason"
		if len(withoutReviewedTraegerTransactions(current)) != len(current)-len(traegerTransactionSites)+1 {
			t.Fatalf("a reworded Träger transaction boundary escaped the shrink-only check: %s", site)
		}
		break
	}
}
