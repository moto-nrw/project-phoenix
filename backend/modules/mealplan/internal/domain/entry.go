package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type Date = timezone.Date

func ParseDate(value string) (Date, error) { return timezone.ParseDate(value) }

type Dish struct {
	Dish string
	Note *string
}

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
