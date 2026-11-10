package test

import "testing"

func TestTenantExportTransactionExceptionIsExact(t *testing.T) {
	t.Parallel()
	current := map[string]string{tenantExportTransactionSite: tenantExportTransactionReason, "unrelated": "unchanged"}
	remaining := withoutReviewedTenantExportTransaction(current)
	if len(remaining) != 1 || remaining["unrelated"] != "unchanged" || len(current) != 2 {
		t.Fatal("reviewed boundary changed another register entry")
	}
	current[tenantExportTransactionSite] = "different reason"
	if len(withoutReviewedTenantExportTransaction(current)) != 2 {
		t.Fatal("changed boundary reason was admitted")
	}
}
