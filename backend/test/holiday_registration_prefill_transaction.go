package test

import "maps"

// holidayRegistrationPrefillTransactionSites is the boundary ADR 0113 adds for
// the parents portal prefill of the holiday registration: the administrative
// lookup of the link. A further site needs its own decision.
var holidayRegistrationPrefillTransactionSites = map[string]string{
	"workflows/parentportal/http/holiday_registration_handlers.go:*Resource.resolveHolidayRegistrationLink:tenant.WithinAdmin#1": "ADR 0113: the parents portal prefill of the holiday registration resolves the Träger and the school IDs of a published period's link before it reads the account's own children; it reads nothing else and writes nothing, like the school slug of the embedded enrollment.",
}

// withoutReviewedHolidayRegistrationPrefillTransactions removes exactly the
// reviewed boundary, with its reviewed reason, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedHolidayRegistrationPrefillTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range holidayRegistrationPrefillTransactionSites {
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
