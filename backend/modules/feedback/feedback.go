// Package feedback is the public Feedback capability.
package feedback

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

const (
	ValuePositive = "positive"
	ValueNeutral  = "neutral"
	ValueNegative = "negative"
)

type Date string

func Today() Date { return Date(timezone.TodayDate().String()) }

func ParseDate(value string) (Date, error) {
	date, err := timezone.ParseDate(value)
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

type engine interface {
	Available(context.Context) (bool, error)
	Submit(context.Context, CreateEntry) (Entry, error)
	SubmitBatch(context.Context, []CreateEntry) ([]Entry, error)
	LookupEntry(context.Context, int64) (Entry, error)
	EraseEntry(context.Context, int64) error
	FindEntries(context.Context, Filter) ([]Entry, error)
	DeleteExpired(context.Context) (int, error)
	CountForStudent(context.Context, int64) (int, error)
	ObserveRejection(string, time.Duration, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("feedback: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) Available(ctx context.Context) (bool, error) { return m.engine.Available(ctx) }

func (m *Module) Submit(ctx context.Context, input CreateEntry) (Entry, error) {
	started := time.Now()
	normalized, err := validateEntry(input)
	if err != nil {
		m.engine.ObserveRejection("submit", time.Since(started), err)
		return Entry{}, err
	}
	return m.engine.Submit(ctx, normalized)
}

func (m *Module) SubmitBatch(ctx context.Context, inputs []CreateEntry) ([]Entry, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	normalized := make([]CreateEntry, 0, len(inputs))
	var failures []error
	for _, input := range inputs {
		entry, err := validateEntry(input)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		normalized = append(normalized, entry)
	}
	if len(failures) > 0 {
		err := &BatchOperationError{Errors: failures}
		m.engine.ObserveRejection("submit_batch", 0, err)
		return nil, err
	}
	return m.engine.SubmitBatch(ctx, normalized)
}

func (m *Module) LookupEntry(ctx context.Context, id int64) (Entry, error) {
	if id <= 0 {
		err := &InvalidEntryDataError{Err: ErrInvalidParameters}
		m.engine.ObserveRejection("read", 0, err)
		return Entry{}, err
	}
	return m.engine.LookupEntry(ctx, id)
}

func (m *Module) EraseEntry(ctx context.Context, id int64) error {
	if id <= 0 {
		err := &InvalidEntryDataError{Err: ErrInvalidParameters}
		m.engine.ObserveRejection("delete", 0, err)
		return err
	}
	return m.engine.EraseEntry(ctx, id)
}

func (m *Module) FindEntries(ctx context.Context, filter Filter) ([]Entry, error) {
	started := time.Now()
	normalized, err := validateFilter(filter)
	if err != nil {
		m.engine.ObserveRejection("read_list", time.Since(started), err)
		return nil, err
	}
	return m.engine.FindEntries(ctx, normalized)
}

func (m *Module) DeleteExpired(ctx context.Context) (int, error) { return m.engine.DeleteExpired(ctx) }

func (m *Module) CountForStudent(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		err := &InvalidEntryDataError{Err: ErrInvalidParameters}
		m.engine.ObserveRejection("count_student", 0, err)
		return 0, err
	}
	return m.engine.CountForStudent(ctx, studentID)
}

func validateEntry(input CreateEntry) (CreateEntry, error) {
	input.Value = strings.TrimSpace(input.Value)
	switch input.Value {
	case ValuePositive, ValueNeutral, ValueNegative:
	default:
		return CreateEntry{}, &InvalidEntryDataError{Err: errors.New("value must be 'positive', 'neutral', or 'negative'")}
	}
	if input.StudentID <= 0 {
		return CreateEntry{}, &InvalidEntryDataError{Err: errors.New("student ID is required")}
	}
	date, err := ParseDate(string(input.Day))
	if err != nil {
		return CreateEntry{}, &InvalidEntryDataError{Err: errors.New("day is required")}
	}
	if _, err := time.Parse("15:04:05", input.Time); err != nil {
		return CreateEntry{}, &InvalidEntryDataError{Err: errors.New("time is required")}
	}
	input.Day = date
	return input, nil
}

func validateFilter(filter Filter) (Filter, error) {
	if filter.StudentID != nil && *filter.StudentID <= 0 {
		return Filter{}, &InvalidEntryDataError{Err: ErrInvalidParameters}
	}
	for _, value := range []*Date{filter.Day, filter.DayFrom, filter.DayTo} {
		if value == nil {
			continue
		}
		parsed, err := ParseDate(string(*value))
		if err != nil {
			return Filter{}, &InvalidEntryDataError{Err: ErrInvalidParameters}
		}
		*value = parsed
	}
	if filter.DayFrom != nil && filter.DayTo != nil {
		start, _ := timezone.ParseDate(string(*filter.DayFrom))
		end, _ := timezone.ParseDate(string(*filter.DayTo))
		if start.After(end) {
			return Filter{}, &InvalidDateRangeError{StartDate: *filter.DayFrom, EndDate: *filter.DayTo}
		}
	}
	return filter, nil
}
