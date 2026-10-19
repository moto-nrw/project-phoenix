package application

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// SupervisionEnded matches ActiveOn: end_date <= today, not a future planned end.
func SupervisionEnded(endDate *string, today string) bool {
	if endDate == nil {
		return false
	}
	end, err := calendar.ParseDate(*endDate)
	if err != nil {
		return false
	}
	day, err := calendar.ParseDate(today)
	if err != nil {
		return false
	}
	return !day.Before(end)
}
