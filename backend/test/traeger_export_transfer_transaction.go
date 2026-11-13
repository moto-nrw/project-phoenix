package test

import "maps"

// traegerExportTransferTransactionSites are the boundaries ADR 0218 adds for
// the Träger scope of Export Transfer: the runtime binding and the one
// Träger transaction boundary of the scope, and the journal's intent API.
// A further site needs its own decision.
var traegerExportTransferTransactionSites = map[string]string{
	"modules/exporttransfer/compose/traeger.go:NewTraegerTransfers:tenant.WithinTraeger#1":                                 "ADR 0218: runtime binding of the Träger scope. NewTraegerTransfers wires the Träger transaction of the tenant runtime into the port; constructing the graph starts no transaction.",
	"modules/exporttransfer/internal/application/traeger_service.go:*TraegerService.within:s.runtime.WithinTraeger#1":      "ADR 0218 section 5: the one boundary of Export Transfer's Träger scope. Each step of a handover (intent with the producer's file and protocol entry, dispatch fence, completion with the producer's settlement) and each recipient read or write commits in the Träger transaction of the session's Träger; it joins the same Träger's request unit and refuses a school or administrative one.",
	"modules/exporttransfer/internal/application/traeger_transfer.go:*TraegerService.beginTraegerIntent:s.journal.Begin#1": "Intent API, not SQL Begin: TraegerJournal.Begin records the durable handover intent of a Träger before external work (ADR 0218). This exact ambiguous-name site stays enumerated so an unrelated SQL Begin cannot acquire the same exception.",
}

// withoutReviewedTraegerExportTransferTransactions removes exactly the
// reviewed boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedTraegerExportTransferTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range traegerExportTransferTransactionSites {
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
