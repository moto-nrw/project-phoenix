package test

import "maps"

// emailDeliveryEventTransactionSites are the reviewed admin units of #5932: a
// reported event names a mail, not a school, so the worker finds the mail and
// its school across schools; the list read follows the claimed mail. Each
// reason must match the register exactly.
var emailDeliveryEventTransactionSites = map[string]string{
	"modules/delivery/internal/application/worker_commands.go:*WorkerCommands.RecordEmailDeliveryEvent:c.transactions.WithinAdmin#1": "Delivery worker command records one delivery event; the mail and thereby its school are found only by the identity in the event.",
	"modules/delivery/internal/application/worker_commands.go:*WorkerCommands.EmailAddressSuppressed:c.transactions.WithinAdmin#1":   "Delivery worker command reads the suppression list of the claimed mail's school.",
	"modules/delivery/internal/application/worker_commands.go:*WorkerCommands.OpenProviderMails:c.transactions.WithinAdmin#1":        "Delivery reconciliation reads the open mails of all schools; a mail is checked by its provider ID, and its school only follows from it.",
	"modules/delivery/internal/application/worker_commands.go:*WorkerCommands.MarkEmailDeliveryChecked:c.transactions.WithinAdmin#1": "Delivery reconciliation notes when it last asked the provider about one mail, across schools.",
}

func withoutReviewedEmailDeliveryEventTransactions(current map[string]string) map[string]string {
	remaining := maps.Clone(current)
	for site, reason := range emailDeliveryEventTransactionSites {
		if current[site] == reason {
			delete(remaining, site)
		}
	}
	return remaining
}
