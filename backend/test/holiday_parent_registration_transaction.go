package test

import "maps"

// holidayParentRegistrationTransactionSites are the boundaries ADR 0216 adds
// for the holiday registrations of a parent account in the parents portal:
// the sign-in's claim by the confirmed address and the list of the account's
// registrations read across Träger with the account in the filter, each in
// one administrative transaction. A further site needs its own decision.
var holidayParentRegistrationTransactionSites = map[string]string{
	"workflows/parentportal/internal/care/holiday_registration_entries.go:*Service.ClaimHolidayRegistrationsAfterSignIn:s.Runtime.WithinAdmin#1": "ADR 0216: after a successful parents-portal sign-in the workflow resolves the account of the signed-in address and links the holiday registrations without an account that name that address, across Träger, in one set-based statement with account and address in its filter. It commits on its own after the sign-in; a failure is logged and never fails the sign-in. It reads no school record.",
	"workflows/parentportal/internal/care/holiday_registration_entries.go:*Service.HolidayRegistrationEntries:s.Runtime.WithinAdmin#1":           "ADR 0216: the parents portal lists the holiday registrations linked to the session's account across Träger, with the account in the filter and a statement count independent of the rows, and names their Träger; it reads no school record and writes nothing.",
}

// withoutReviewedHolidayParentRegistrationTransactions removes exactly the
// reviewed boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedHolidayParentRegistrationTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range holidayParentRegistrationTransactionSites {
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
