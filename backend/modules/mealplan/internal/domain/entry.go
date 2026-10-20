package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Date = calendar.Date

type Entry struct {
	Date     Date
	Position int
	Dish     string
	Note     *string
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}
