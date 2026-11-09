package test

import "maps"

// belegRetentionTransactionSites are the boundaries ADR 0184 adds for the
// Beleg retention of a Träger in the Operator portal (#4720): the overview
// and each write run in one administrative transaction, the write together
// with its operator audit entry. A further site needs its own decision.
var belegRetentionTransactionSites = map[string]string{
	"modules/parentbilling/http/beleg_retention.go:*BelegRetentionResource.Overview:tenant.WithinAdmin#1": "ADR 0184: the operator's overview of the Beleg retention of the one Träger in the route reads its instructions, suspensions, counts, clarification list and deletion log; the tables have no request-role privilege, and the read is in the operator audit.",
	"modules/parentbilling/http/beleg_retention.go:*BelegRetentionResource.write:tenant.WithinAdmin#1":    "ADR 0184: an operator's instruction, suspension or lift for the one Träger in the route commits together with its operator audit entry and the re-evaluation of that Träger's Belege; it deletes nothing.",
}

// withoutReviewedBelegRetentionTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedBelegRetentionTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range belegRetentionTransactionSites {
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
