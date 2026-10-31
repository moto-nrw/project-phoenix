package test

import "maps"

// schoolVacationTransactionSites are the boundaries ADR 0143 adds for the
// school vacations: the administrative transaction of the daily sync, which
// renews the vacations of every federal state for all schools at once. A
// further site needs its own decision.
var schoolVacationTransactionSites = map[string]string{
	"modules/schoolcalendar/compose/new.go:NewSchoolVacations:tenant.WithinAdmin#1": "ADR 0143: the daily school vacation sync replaces the vacations of every federal state in calendar.school_vacations, a table without a school that every school reads; it runs once for all schools after the request to the OpenHolidays API and writes nothing else.",
}

// withoutReviewedSchoolVacationTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedSchoolVacationTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range schoolVacationTransactionSites {
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
