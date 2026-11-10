package test

import "testing"

func TestAccountMFAEnrollmentTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{accountMFAEnrollmentTransactionSite: accountMFAEnrollmentTransactionReason, "unrelated": "unchanged"}
	remaining := withoutReviewedAccountMFAEnrollmentTransaction(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != 2 {
		t.Fatal("reviewed boundary changed another register entry")
	}
	current[accountMFAEnrollmentTransactionSite] = "different reason"
	if len(withoutReviewedAccountMFAEnrollmentTransaction(current)) != 2 {
		t.Fatal("changed boundary reason was admitted")
	}
}
