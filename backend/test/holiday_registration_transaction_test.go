package test

import "testing"

func TestHolidayRegistrationTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range holidayRegistrationTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedHolidayRegistrationTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed holiday registration boundaries were not removed exactly: %#v", remaining)
	}
	for site := range holidayRegistrationTransactionSites {
		current[site] = "different reason"
		if len(withoutReviewedHolidayRegistrationTransactions(current)) != len(current)-len(holidayRegistrationTransactionSites)+1 {
			t.Fatalf("a reworded holiday registration boundary escaped the shrink-only check: %s", site)
		}
		break
	}
}
