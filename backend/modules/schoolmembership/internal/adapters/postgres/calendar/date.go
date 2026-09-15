// Package calendar maps the module's YYYY-MM-DD strings onto the repository
// calendar-date type so DATE columns bind without a timezone shift.
package calendar

import "github.com/moto-nrw/project-phoenix/internal/timezone"

type Date = timezone.Date

func ParseDate(value string) (Date, error) {
	return timezone.ParseDate(value)
}
