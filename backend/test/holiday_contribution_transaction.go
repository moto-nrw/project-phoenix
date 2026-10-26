package test

import "maps"

// holidayContributionTransactionSites are the boundaries ADR 0115 adds for the
// public contribution of the holiday registration: the administrative lookup
// of its link and the Träger transaction of the link's Träger it computes in.
// A further site needs its own decision.
var holidayContributionTransactionSites = map[string]string{
	"modules/parentbilling/http/holiday.go:*HolidayResource.resolveLink:tenant.WithinAdmin#1": "ADR 0115: the public contribution of the holiday registration resolves the Träger and the published period of its link before any Träger context exists, like the registration form; it reads nothing else and writes nothing.",
	"modules/parentbilling/http/holiday.go:*HolidayResource.quote:tenant.WithinTraeger#1":     "ADR 0115: the public contribution of the holiday registration reads the period's prices and the Träger's rules in the Träger transaction of the Träger its resolved link names; it stores nothing and the Träger never comes from the request.",
}

// withoutReviewedHolidayContributionTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedHolidayContributionTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range holidayContributionTransactionSites {
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
