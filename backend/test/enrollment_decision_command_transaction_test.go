package test

import "testing"

func TestEnrollmentDecisionCommandTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{enrollmentDecisionCommandTransactionSite: enrollmentDecisionCommandTransactionReason, "unrelated": "unchanged"}
	remaining := withoutReviewedEnrollmentDecisionCommandTransaction(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != 2 {
		t.Fatal("reviewed boundary changed another register entry")
	}
	current[enrollmentDecisionHandlerTransactionSite] = "kept handler boundary"
	if len(withoutReviewedEnrollmentDecisionCommandTransaction(current)) != 3 {
		t.Fatal("owner boundary was admitted beside the handler boundary it replaces")
	}
	delete(current, enrollmentDecisionHandlerTransactionSite)
	current[enrollmentDecisionCommandTransactionSite] = "different reason"
	if len(withoutReviewedEnrollmentDecisionCommandTransaction(current)) != 2 {
		t.Fatal("changed boundary reason was admitted")
	}
}
