package calendar

import "github.com/moto-nrw/project-phoenix/internal/timezone"

type Date = timezone.Date

func ParseDate(value string) (Date, error) {
	return timezone.ParseDate(value)
}
