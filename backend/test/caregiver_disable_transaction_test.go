package test

import "testing"

func TestCaregiverDisableTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{caregiverDisableTransactionSite: caregiverDisableTransactionReason, "unrelated": "unchanged"}
	remaining := withoutReviewedCaregiverDisableTransaction(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != 2 {
		t.Fatal("reviewed boundary changed another register entry")
	}
	current[caregiverDisableTransactionSite] = "different reason"
	if len(withoutReviewedCaregiverDisableTransaction(current)) != 2 {
		t.Fatal("changed boundary reason was admitted")
	}
}
