package test

import "maps"

// traegerStaffTransactionSites are the boundaries ADR 0089 decides for the
// Personalstamm of the Träger (#4705): the Träger-Portal reads and writes the
// records in the Träger transaction of the session's Träger after it read
// each school in that school's own tenant transaction, never both at once
// (ADR 0105); the moto team grants and withdraws the release in one
// administrative transaction with its operator audit entry. A further site
// needs its own decision.
var traegerStaffTransactionSites = map[string]map[string]string{
	"admin": {
		"modules/workforce/http/traegerstaff/release.go:*ReleaseResource.State:tenant.WithinAdmin#1": "ADR 0089 section 2: the operator's view of the release „Personal beim Träger“ of the one Träger in the route reads the release and the number of its staff records; the read is in the operator audit and writes nothing.",
		"modules/workforce/http/traegerstaff/release.go:*ReleaseResource.write:tenant.WithinAdmin#1": "ADR 0089 section 2: an operator's grant of the release with the statement the Träger confirmed, or its withdrawal, commits together with its operator audit entry; the records' foreign key refuses a withdrawal while the Träger keeps records.",
	},
	"commands": {
		"modules/workforce/internal/application/traeger_staff.go:*TraegerStaff.Overview:t.ports.Runtime.WithinTraeger#1": "ADR 0089 sections 3 and 5: the overview reads the release and the staff records of the session's Träger in its Träger transaction before each school's employments are read and logged in that school's tenant transaction; no transaction carries both and it writes nothing.",
		"modules/workforce/internal/application/traeger_staff.go:*TraegerStaff.released:t.ports.Runtime.WithinTraeger#1": "ADR 0089 sections 3, 5 and 6: a read or write of one staff record of the session's Träger runs in its Träger transaction after the release check; a write locks the record, its links and its trail commit together, and the schools are read or brought up to date only in their own tenant transactions before or after it.",
	},
}

// withoutReviewedTraegerStaffTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedTraegerStaffTransactions(current map[string]string) map[string]string {
	remaining := current
	for _, sites := range traegerStaffTransactionSites {
		for site, reason := range sites {
			if current[site] != reason {
				continue
			}
			if len(remaining) == len(current) {
				remaining = maps.Clone(current)
			}
			delete(remaining, site)
		}
	}
	return remaining
}
