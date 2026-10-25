package test

import "maps"

// traegerViewTransactionSites are the administrative boundaries ADR 0084
// section 1 admits for the Träger-Portal: the Träger projection reads the
// schools of one Träger in one administrative transaction, and nothing else
// opens one with Träger reference. The projection's scope ratchet
// (traeger_view_scope_ratchet_test.go) holds the filter on every query.
var traegerViewTransactionSites = map[string]string{
	"modules/traegerview/compose/compose.go:New:tenant.WithinAdmin#1":                        "ADR 0084 section 1: the Träger projection composition binds its one administrative read unit; every query in its Postgres package filters on the schools of the Träger, held by TestTraegerViewQueriesFilterOnTheTraegerSchools.",
	"modules/traegerview/internal/application/overview.go:*Overview.Schools:o.withinAdmin#1": "ADR 0084 section 1: the Träger overview reads the school set of the session's Träger and its rows in one administrative unit; it never writes and opens no tenant transaction.",
}

// withoutReviewedTraegerViewTransactions removes exactly the reviewed Träger
// projection boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedTraegerViewTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range traegerViewTransactionSites {
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
