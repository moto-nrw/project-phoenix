package test

import "maps"

// holidayClosureNotificationTransactionSites are the boundaries ADR 0196
// adds: the administrative read unit in which the worker of the holiday
// closure notices asks the narrow definer function which Träger have due
// commands. It reads Träger IDs only, writes nothing, and ends before the
// Träger transaction that processes the commands.
var holidayClosureNotificationTransactionSites = map[string]string{
	"roots/owners/holiday_closure_notification.go:holidayClosureUnits.WithinAdmin:tenant.WithinAdmin#1": "ADR 0196: the worker's unit binding opens one administrative read unit, masking the caller's school and transaction; the workflow uses it only to list the Träger with due holiday closure commands.",
	"workflows/holidayclosurenotification/worker.go:*Service.ProcessDue:s.Units.WithinAdmin#1":          "ADR 0196: the worker lists the Träger with due holiday closure commands through calendar.holiday_closure_due_traeger_ids() in one administrative read unit; it writes nothing and ends before each Träger's own transaction.",
}

// withoutReviewedHolidayClosureNotificationTransactions removes exactly the
// reviewed boundaries, with their reviewed reasons, before the shrink-only
// comparison. Any other new or reworded site stays.
func withoutReviewedHolidayClosureNotificationTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range holidayClosureNotificationTransactionSites {
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
