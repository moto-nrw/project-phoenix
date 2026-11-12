package test

import "maps"

// holidayParentSetupTransactionSites are the boundaries ADR 0216 adds for
// the setup link of a parent account at a Träger: the link carries no school
// and no session, so its preview and its redemption read the global account
// and the Träger, and the redemption writes the account and its access at the
// Träger, in one administrative transaction. A further site needs its own
// decision.
var holidayParentSetupTransactionSites = map[string]string{
	"modules/identityaccess/internal/application/holiday_parent_setup.go:*AccountLifecycle.CompleteHolidayParentSetup:l.runtime.WithAdminTx#1": "ADR 0216: the setup link of a parent account at a Träger names an address and a Träger and no school; its redemption locks the global account by address, creates it with its password when the address has none, locks the Träger and records the access at the Träger, atomically. It grants no school, profile, role or child.",
	"modules/identityaccess/internal/application/holiday_parent_setup.go:*AccountLifecycle.PreviewHolidayParentSetup:l.runtime.WithAdminTx#1":  "ADR 0216: the setup page of a parent account at a Träger reads the Träger's name and whether the link's address has a global account; it reads no school record and writes nothing.",
}

// withoutReviewedHolidayParentSetupTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only comparison.
// It is no general escape: any other new or reworded site stays.
func withoutReviewedHolidayParentSetupTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range holidayParentSetupTransactionSites {
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
