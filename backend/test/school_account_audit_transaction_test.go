package test

import "testing"

func TestSchoolAccountAuditTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{"unrelated": "unchanged"}
	for site, reason := range schoolAccountAuditTransactionSites {
		current[site] = reason
	}
	remaining := withoutReviewedSchoolAccountAuditTransactions(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != len(schoolAccountAuditTransactionSites)+1 {
		t.Fatal("reviewed boundaries changed another register entry")
	}
	for site := range schoolAccountAuditTransactionSites {
		current[site] = "different reason"
	}
	if len(withoutReviewedSchoolAccountAuditTransactions(current)) != len(current) {
		t.Fatal("changed boundary reason was admitted")
	}
}
