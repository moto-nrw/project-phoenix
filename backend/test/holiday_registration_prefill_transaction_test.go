package test

import "testing"

func TestHolidayRegistrationPrefillTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range holidayRegistrationPrefillTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedHolidayRegistrationPrefillTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" {
		t.Fatalf("reviewed holiday registration prefill boundary was not removed exactly: %#v", remaining)
	}
	for site := range holidayRegistrationPrefillTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedHolidayRegistrationPrefillTransactions(current)) != len(current) {
		t.Fatal("a reworded holiday registration prefill boundary escaped the shrink-only check")
	}
}
