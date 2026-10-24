package calendar

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

type Date = calendar.Date

func ParseDate(value string) (Date, error) {
	return calendar.ParseDate(value)
}
