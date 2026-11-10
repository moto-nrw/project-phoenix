package test

import "testing"

func TestEnrollmentFamilyChildrenTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	count := 0
	for _, sites := range enrollmentFamilyChildrenTransactionSites {
		for site, reason := range sites {
			current[site] = reason
			count++
		}
	}
	remaining := withoutReviewedEnrollmentFamilyChildrenTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != count+1 {
		t.Fatal("reviewed boundaries changed another register entry")
	}
	for _, sites := range enrollmentFamilyChildrenTransactionSites {
		for site := range sites {
			current[site] = "different reason"
		}
	}
	if len(withoutReviewedEnrollmentFamilyChildrenTransactions(current)) != len(current) {
		t.Fatal("changed boundary reason was admitted")
	}
}
