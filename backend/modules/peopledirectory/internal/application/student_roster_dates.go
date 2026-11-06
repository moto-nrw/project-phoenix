package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListStudentRosterOnDates reuses the enrollment-aware range read, including
// its health decoding and holiday-guest gate. Its result is a superset of the
// point reads; each day still applies its own enrollment start and end.
func (s *Service) ListStudentRosterOnDates(ctx context.Context, dates []string, today string) (map[string][]peopledirectory.StudentRosterEntry, error) {
	result := make(map[string][]peopledirectory.StudentRosterEntry, len(dates))
	if len(dates) == 0 {
		return result, nil
	}
	if err := peopledirectory.ValidateRosterDay(today); err != nil {
		return nil, err
	}
	from, to := dates[0], dates[0]
	for _, date := range dates {
		if err := peopledirectory.ValidateRosterDay(date); err != nil {
			return nil, err
		}
		from, to = min(from, date), max(to, date)
	}
	values, err := s.students.ListRosterOverlapping(ctx, from, to, today)
	if err != nil {
		return nil, MapError(err)
	}
	roster := toPublicRoster(values)
	for _, date := range dates {
		entries := make([]peopledirectory.StudentRosterEntry, 0, len(roster))
		for _, value := range roster {
			if peopledirectory.EnrolledOn(value.Record, calendar.Date(date), calendar.Date(today)) {
				entries = append(entries, value)
			}
		}
		result[date] = entries
	}
	return result, nil
}
