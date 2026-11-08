package test

import "maps"

// enrollmentFormLabelsTransactionSites are the read boundaries #6142 adds so
// the change request view names its rows with the request's form questions.
// The change request routes keep their exact query budgets; the labels are
// read in their own request. A further site needs its own decision.
var enrollmentFormLabelsTransactionSites = map[string]map[string]string{
	"admin": {
		"modules/enrollment/http/form_labels_handlers.go:*Resource.getPublicFormLabels:withinAdmin#1": "Public status capability-token read of the request's form labels (#6142) identifies the request and school before school middleware exists; token validation stays mandatory and the schema and legal-block reads run in the resolved school's scope. It writes nothing.",
	},
	"commands": {
		"modules/enrollment/http/form_labels_handlers.go:*Resource.getAdminFormLabels:rs.runInTenantTx#1": "HTTP query boundary: getAdminFormLabels reads the request's pinned form labels (#6142) in the school's tenant transaction, so a request of another school is not found under RLS. It writes nothing.",
	},
}

// withoutReviewedEnrollmentFormLabelsTransactions removes exactly the
// reviewed boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedEnrollmentFormLabelsTransactions(current map[string]string) map[string]string {
	remaining := current
	for _, sites := range enrollmentFormLabelsTransactionSites {
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
