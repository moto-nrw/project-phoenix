package test

import "testing"

func TestSchoolVacationTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range schoolVacationTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedSchoolVacationTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed school vacation boundaries were not removed exactly: %#v", remaining)
	}
	for site := range schoolVacationTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedSchoolVacationTransactions(current)) != len(current) {
		t.Fatal("a reworded school vacation boundary escaped the shrink-only check")
	}
}
