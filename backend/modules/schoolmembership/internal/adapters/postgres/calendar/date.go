// Package calendar maps the module's YYYY-MM-DD strings onto the repository
// calendar-date type so DATE columns bind without a timezone shift.
package calendar

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Date = calendar.Date

func ParseDate(value string) (Date, error) {
	return calendar.ParseDate(value)
}

func TodayDate() Date {
	return calendar.TodayDate()
}

// DateFromTime maps an instant to its Berlin calendar date for DATE queries.
func DateFromTime(value time.Time) Date {
	return calendar.DateFromTime(value)
}
