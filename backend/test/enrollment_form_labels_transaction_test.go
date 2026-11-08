package test

import "testing"

func TestEnrollmentFormLabelsTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	count := 0
	for _, sites := range enrollmentFormLabelsTransactionSites {
		for site, reason := range sites {
			current[site] = reason
			count++
		}
	}
	remaining := withoutReviewedEnrollmentFormLabelsTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != count+1 {
		t.Fatal("reviewed boundaries changed another register entry")
	}
	for _, sites := range enrollmentFormLabelsTransactionSites {
		for site := range sites {
			current[site] = "different reason"
		}
	}
	if len(withoutReviewedEnrollmentFormLabelsTransactions(current)) != len(current) {
		t.Fatal("changed boundary reason was admitted")
	}
}
