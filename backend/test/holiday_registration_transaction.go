package test

import "maps"

// holidayRegistrationTransactionSites are the boundaries ADR 0110 adds for the
// public holiday registration: the administrative lookup of its link, the
// public routes' entry into the Träger transaction and the three units of the
// form, the abuse counter and the registration, and the two units of the
// status link of a registration (#6676). A further site needs its own
// decision.
var holidayRegistrationTransactionSites = map[string]string{
	"modules/enrollment/http/holiday_form_documents.go:*HolidayRegistrationResource.document:withinTraeger#1":    "ADR 0110 #5199: public PDF reads use only the resolved link's Träger and current published form references; no registration or personal data is read.",
	"modules/enrollment/http/holiday_registration_status.go:*HolidayRegistrationResource.status:withinAdmin#1":   "ADR 0110 #6676: the status link of a holiday registration resolves the Träger and the registration ID of its token, and the Träger's school names, before any Träger context exists, like the link of a period; it reads nothing else and writes nothing.",
	"modules/enrollment/http/holiday_registration_status.go:*HolidayRegistrationResource.status:withinTraeger#1": "ADR 0110 #6676: the status link reads the state, period, first names and booked weeks of its resolved registration under the Träger policy; it writes nothing.",
	"modules/enrollment/http/holiday_registrations.go:*HolidayRegistrationResource.resolveLink:withinAdmin#1":    "ADR 0110: the public holiday registration link resolves its Träger, published period and the Träger's school names before any Träger context exists, like the school slug of the online registration; it reads nothing else and writes nothing.",
	"modules/enrollment/http/transactions.go:withinTraeger:tenant.WithinTraeger#1":                               "ADR 0110: the public enrollment routes open the Träger transaction of the Träger their resolved link names; the Träger never comes from the request.",
	"modules/enrollment/http/holiday_registrations.go:*HolidayRegistrationResource.offer:withinTraeger#1":        "ADR 0110: the public holiday registration reads the published period of its resolved link under the Träger policy.",
	"modules/enrollment/http/holiday_registrations.go:*HolidayRegistrationResource.countAttempt:withinTraeger#1": "ADR 0110: the abuse counter of the public holiday registration commits in a Träger transaction of its own, so a refused registration still counts.",
	"modules/enrollment/http/holiday_registrations.go:*HolidayRegistrationResource.submit:withinTraeger#1":       "ADR 0110: the public holiday registration stores the registration and its receipt mail in one Träger transaction of the link's Träger.",
}

// withoutReviewedHolidayRegistrationTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only comparison.
// It is no general escape: any other new or reworded site stays.
func withoutReviewedHolidayRegistrationTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range holidayRegistrationTransactionSites {
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
