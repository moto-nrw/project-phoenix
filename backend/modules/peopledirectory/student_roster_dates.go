package peopledirectory

import (
	"context"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// StudentRostersOnDates reuses the enrollment-aware range read, including
// its health decoding and holiday-guest gate. Its result is a superset of the
// point reads; each day still applies its own enrollment start and end.
func StudentRostersOnDates(ctx context.Context, source Capability, dates []string, today string) (map[string][]StudentRosterEntry, error) {
	result := make(map[string][]StudentRosterEntry, len(dates))
	if len(dates) == 0 {
		return result, nil
	}
	if err := ValidateRosterDay(today); err != nil {
		return nil, err
	}
	from, to := dates[0], dates[0]
	for _, date := range dates {
		if err := ValidateRosterDay(date); err != nil {
			return nil, err
		}
		from, to = min(from, date), max(to, date)
	}
	roster, err := source.ListStudentRosterOverlapping(ctx, from, to, today)
	if err != nil {
		return nil, err
	}
	for _, date := range dates {
		entries := make([]StudentRosterEntry, 0, len(roster))
		for _, value := range roster {
			if EnrolledOn(value.Record, calendar.Date(date), calendar.Date(today)) {
				entries = append(entries, value)
			}
		}
		result[date] = entries
	}
	return result, nil
}
