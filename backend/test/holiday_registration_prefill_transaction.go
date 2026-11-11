package test

import "maps"

// holidayRegistrationPrefillTransactionSites is the boundary ADR 0113 adds for
// the parents portal prefill of the holiday registration: the administrative
// lookup of the link. ADR 0207 adds the lookup of the published holiday
// periods the start page of the parents app announces (#6671). A further
// site needs its own decision.
var holidayRegistrationPrefillTransactionSites = map[string]string{
	"workflows/parentportal/http/holiday_registration_handlers.go:*Resource.resolveHolidayRegistrationLink:tenant.WithinAdmin#1":     "ADR 0113: the parents portal prefill of the holiday registration resolves the Träger and the school IDs of a published period's link before it reads the account's own children; it reads nothing else and writes nothing, like the school slug of the embedded enrollment.",
	"workflows/parentportal/internal/care/holiday_registration_offers.go:*Service.HolidayRegistrationOffers:s.Runtime.WithinAdmin#1": "ADR 0207: the start page of the parents app resolves the account's children and their relationship-level enrollment.submit permission across its schools, the active Träger of those schools and the published holiday periods of exactly those Träger; it reads no school record and writes nothing.",
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
