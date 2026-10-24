package test

import "testing"

func TestTraegerViewTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range traegerViewTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedTraegerViewTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed Träger projection boundaries were not removed exactly: %#v", remaining)
	}
	for site := range traegerViewTransactionSites {
		current[site] = "different reason"
		if len(withoutReviewedTraegerViewTransactions(current)) != len(current)-len(traegerViewTransactionSites)+1 {
			t.Fatalf("a reworded Träger projection boundary escaped the shrink-only check: %s", site)
		}
		break
	}
}
