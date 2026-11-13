package test

import "maps"

// schoolAccountAuditTransactionSites are the boundaries #6984 adds so the
// operator audit of a school account commits before the account's school
// unit: the tenant role cannot write platform.operator_audit_log, and an
// audit failure must roll the account back. A further site needs its own
// decision.
var schoolAccountAuditTransactionSites = map[string]string{
	"modules/organizationtenancy/internal/adapters/postgres/transaction.go:Transaction.RunAdminDetached:tenant.WithinAdmin#1":             "Organization provisioning command RunAdminDetached masks an ambient school unit and commits one administrative unit of its own before that school unit commits; only the operator audit of a school account creation uses it (#6984).",
	"modules/organizationtenancy/internal/application/provisioning_accounts.go:*Provisioning.CreateSchoolAccount:p.tx.RunAdminDetached#1": "Organization provisioning command CreateSchoolAccount commits its operator audit entry in an administrative unit of its own while the school unit is open; the tenant role cannot write platform.operator_audit_log, and an audit failure rolls the account back (#6984).",
}

// withoutReviewedSchoolAccountAuditTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedSchoolAccountAuditTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range schoolAccountAuditTransactionSites {
		if current[site] != reason {
			continue
		}
		if len(remaining) == len(current) {
			remaining = maps.Clone(current)
		}
		delete(remaining, site)
	}
	return remaining
}
