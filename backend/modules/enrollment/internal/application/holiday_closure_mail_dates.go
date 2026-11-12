package application

import (
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// holidayClosureMailHasPastDays evaluates the date of dispatch, including
// persisted mail from before ISO dates were added to its payload.
func holidayClosureMailHasPastDays(mail enrollment.HolidayClosureMail, today calendar.Date) bool {
	for _, children := range [][]enrollment.HolidayClosureMailChild{mail.Children, mail.ReopenedChildren} {
		for _, child := range children {
			for _, week := range child.Weeks {

				for _, date := range holidayClosureMailWeekDates(week) {
					if date.Before(today) {
						return true
					}
				}
			}
		}
	}
	return false
}

func holidayClosureMailWeekDates(week enrollment.HolidayClosureMailWeek) []calendar.Date {
	if len(week.Dates) > 0 {
		return week.Dates
	}
	var dates []calendar.Date
	for _, label := range week.ClosedDays {
		_, date, ok := strings.Cut(label, ", ")
		parts := strings.Split(date, ".")
		if !ok || len(parts) != 3 {
			continue
		}
		if parsed, err := calendar.ParseDate(parts[2] + "-" + parts[1] + "-" + parts[0]); err == nil {
			dates = append(dates, parsed)
		}
	}
	return dates
}
