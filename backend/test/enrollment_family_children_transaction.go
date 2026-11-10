package test

import "maps"

// enrollmentFamilyChildrenTransactionSites are the reviewed boundaries of
// the children of the family on a registration's status page (#6335).
var enrollmentFamilyChildrenTransactionSites = map[string]map[string]string{
	"admin": {
		"modules/enrollment/internal/application/intake_family_children.go:*Intake.familyChildren:s.deps.Runtime.AdminTx#1": "Public enrollment status capability-token bootstrap identifies the request and school before school middleware exists (#6335); token validation stays mandatory and the number is read and written in the resolved school transaction.",
	},
	"commands": {
		"modules/enrollment/internal/application/intake_family_children.go:*Intake.familyChildren:s.deps.Runtime.TenantTx#1": "Atomic owner command (#6335): the status page's number of children of the family is read under the request lock, checked against the open children and written in one school transaction; SQL adapters below it require that unit and cannot open one.",
	},
}

// withoutReviewedEnrollmentFamilyChildrenTransactions removes exactly the
// reviewed boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedEnrollmentFamilyChildrenTransactions(current map[string]string) map[string]string {
	remaining := current
	for _, sites := range enrollmentFamilyChildrenTransactionSites {
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
