package test

import "maps"

// billingKeyDateTransactionSites are the boundaries #5715 adds so the billing
// key-date capture replays after a serialization conflict with the Worker's
// lease renewal. A further site needs its own decision.
var billingKeyDateTransactionSites = map[string]string{
	"modules/organizationtenancy/internal/adapters/postgres/transaction.go:Transaction.RunAdminRetry:tenant.WithinAdminRetry#1": "Organization directory/provisioning command RunAdminRetry runs one retry-safe administrative command and replays the whole transaction after a deadlock or serialization failure; only commands that re-read everything they write may use it.",
	"modules/organizationtenancy/internal/application/billing.go:*Billing.RecordDueBillingKeyDates:b.tx.RunAdminRetry#1":        "Operator/worker billing command RecordDueBillingKeyDates records platform-wide school counts at the configured billing key day in one repeatable-read snapshot; it replays on a fresh snapshot when the Worker's lease renewal conflicts with its commit fence (#5715), and each row retains its explicit school identity.",
}

// withoutReviewedBillingKeyDateTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedBillingKeyDateTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range billingKeyDateTransactionSites {
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
