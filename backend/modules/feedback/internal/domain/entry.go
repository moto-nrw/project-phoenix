package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Date = calendar.Date

func ParseDate(value string) (Date, error) { return calendar.ParseDate(value) }

type Entry struct {
	ID              int64
	Value           string
	Day             Date
	Time            time.Time
	StudentID       int64
	IsMensaFeedback bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Filter struct {
	StudentID       *int64
	Day             *Date
	IsMensaFeedback *bool
	DayFrom         *Date
	DayTo           *Date
	ValueLike       *string
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}
