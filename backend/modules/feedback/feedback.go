// Package feedback is the public Feedback capability.
package feedback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const (
	ValuePositive = "positive"
	ValueNeutral  = "neutral"
	ValueNegative = "negative"
)

type Date string

func Today() Date { return Date(calendar.TodayDate().String()) }

// TimestampParts returns the Berlin calendar day and wall-clock time stored
// for one feedback instant, including across midnight and DST transitions.
func TimestampParts(instant time.Time) (Date, string) {
	local := instant.In(calendar.Berlin)
	return Date(calendar.DateFromTime(local).String()), local.Format("15:04:05")
}

func ParseDate(value string) (Date, error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return "", err
	}
	return Date(date.String()), nil
}

type Student struct {
	ID        int64
	FirstName string
	LastName  string
}

type Entry struct {
	ID              int64
	Value           string
	Day             Date
	Time            string
	StudentID       int64
	IsMensaFeedback bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Student         *Student
}

type CreateEntry struct {
	Value           string
	Day             Date
	Time            string
	StudentID       int64
	IsMensaFeedback bool
}

type Filter struct {
	StudentID       *int64
	Day             *Date
	IsMensaFeedback *bool
	DayFrom         *Date
	DayTo           *Date
	ValueLike       *string
}

var (
	ErrFeatureDisabled   = errors.New("feature_disabled")
	ErrEntryNotFound     = errors.New("feedback entry not found")
	ErrInvalidEntryData  = errors.New("invalid feedback entry data")
	ErrStudentNotFound   = errors.New("student not found")
	ErrInvalidDateRange  = errors.New("invalid date range")
	ErrInvalidParameters = errors.New("invalid parameters")
)

type EntryNotFoundError struct{ EntryID int64 }

func (e *EntryNotFoundError) Error() string {
	return fmt.Sprintf("feedback entry not found: %d", e.EntryID)
}
func (e *EntryNotFoundError) Unwrap() error { return ErrEntryNotFound }

type InvalidEntryDataError struct{ Err error }

func (e *InvalidEntryDataError) Error() string {
	return fmt.Sprintf("invalid feedback entry data: %v", e.Err)
}
func (e *InvalidEntryDataError) Unwrap() error { return ErrInvalidEntryData }

type InvalidDateRangeError struct {
	StartDate Date
	EndDate   Date
}

func (e *InvalidDateRangeError) Error() string {
	return fmt.Sprintf("invalid date range: %s to %s", e.StartDate, e.EndDate)
}
func (e *InvalidDateRangeError) Unwrap() error { return ErrInvalidDateRange }

type BatchOperationError struct{ Errors []error }

func (e *BatchOperationError) Error() string {
	return fmt.Sprintf("batch operation failed with %d errors", len(e.Errors))
}
func (e *BatchOperationError) Unwrap() []error { return e.Errors }

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrFeatureDisabled):
		return "feature_disabled"
	case errors.Is(err, ErrEntryNotFound):
		return "entry_not_found"
	case errors.Is(err, ErrInvalidEntryData):
		return "invalid_entry_data"
	case errors.Is(err, ErrStudentNotFound):
		return "student_not_found"
	case errors.Is(err, ErrInvalidDateRange):
		return "invalid_date_range"
	case errors.Is(err, ErrInvalidParameters):
		return "invalid_parameters"
	default:
		return "internal_error"
	}
}

type Capability interface {
	Available(context.Context) (bool, error)
	Submit(context.Context, CreateEntry) (Entry, error)
	SubmitBatch(context.Context, []CreateEntry) ([]Entry, error)
	LookupEntry(context.Context, int64) (Entry, error)
	EraseEntry(context.Context, int64) error
	FindEntries(context.Context, Filter) ([]Entry, error)
	DeleteExpired(context.Context) (int, error)
	CountForStudent(context.Context, int64) (int, error)
}
