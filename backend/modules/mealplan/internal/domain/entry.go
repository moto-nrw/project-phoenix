package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Date = calendar.Date

func ParseDate(value string) (Date, error) { return calendar.ParseDate(value) }

// DateFromInstant is the Berlin calendar day of an instant.
func DateFromInstant(instant time.Time) Date { return calendar.DateFromTime(instant) }

type Entry struct {
	Date      Date
	Position  int
	Dish      string
	Note      *string
	Allergens []string
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}
