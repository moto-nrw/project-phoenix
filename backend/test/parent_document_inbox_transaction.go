package test

import "maps"

// parentDocumentInboxTransactionSites are the boundaries ADR 0126 adds for
// the document inbox of the parents portal: the administrative lookup of the
// account's children, their Träger and the account's address, and the same
// lookup for the push of a new document. A further site needs its own
// decision.
var parentDocumentInboxTransactionSites = map[string]string{
	"workflows/parentportal/internal/care/billing_documents.go:*Service.billingDocumentAccess:s.Runtime.WithinAdmin#1": "ADR 0126: the parents portal inbox resolves the account's children and relationship-level permissions across its schools, the Träger of those schools and the account's current address before it reads that Träger's documents in the Träger transaction; it writes nothing.",
	"workflows/parentportal/internal/care/billing_documents.go:*Service.documentRecipient:s.Runtime.WithinAdmin#1":     "ADR 0126: the push of a new document resolves the account of the recipient's address, its children and relationship-level permissions across its schools and their Träger before it notifies each permitted school in that school's tenant transaction; it writes nothing.",
}

// withoutReviewedParentDocumentInboxTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedParentDocumentInboxTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range parentDocumentInboxTransactionSites {
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
