package test

import "maps"

// realtimeNotificationTransactionSites are the two exact Delivery boundaries
// accepted by ADR 0146. No other site or changed rationale is exempt.
var realtimeNotificationTransactionSites = map[string]string{
	"modules/delivery/internal/notifications/sse_relay.go:*sseChannel.relayable:tenant.WithTenantTx#1":   "ADR 0146: Delivery stores the short-lived notification copy after the producer commits, in the explicitly addressed school under RLS, bounded by the 250 ms storage deadline; failure keeps delivery local.",
	"modules/delivery/internal/notifications/sse_relay.go:*sseChannel.loadRelayed:tenant.WithTenantTx#1": "ADR 0146: Delivery loads the short-lived notification copy in the signal school under RLS before matching its stored recipients; guardian delivery additionally reuses the existing relationship-permission boundary.",
}

func withoutReviewedRealtimeNotificationTransactions(current map[string]string) map[string]string {
	remaining := maps.Clone(current)
	for site, reason := range realtimeNotificationTransactionSites {
		if current[site] == reason {
			delete(remaining, site)
		}
	}
	return remaining
}
