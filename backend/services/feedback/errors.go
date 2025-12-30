package feedback

import (
	"errors"
	"fmt"
	"time"
)

// Common error types
var (
	ErrEntryNotFound     = errors.New("feedback entry not found")
	ErrInvalidEntryData  = errors.New("invalid feedback entry data")
	ErrStudentNotFound   = errors.New("student not found")
	ErrInvalidDateRange  = errors.New("invalid date range")
	ErrInvalidParameters = errors.New("invalid parameters")
)

// EntryNotFoundError wraps an entry not found error
type EntryNotFoundError struct {
	EntryID int64
}

func (e *EntryNotFoundError) Error() string {
	return fmt.Sprintf("feedback entry not found: %d", e.EntryID)
}

func (e *EntryNotFoundError) Unwrap() error {
	return ErrEntryNotFound
}

// InvalidEntryDataError wraps a validation error for an entry
type InvalidEntryDataError struct {
	Err error
}

func (e *InvalidEntryDataError) Error() string {
	return fmt.Sprintf("invalid feedback entry data: %v", e.Err)
}

func (e *InvalidEntryDataError) Unwrap() error {
	return ErrInvalidEntryData
}

// InvalidDateRangeError wraps an invalid date range error
type InvalidDateRangeError struct {
	StartDate time.Time
	EndDate   time.Time
}

func (e *InvalidDateRangeError) Error() string {
	return fmt.Sprintf("invalid date range: %s to %s", e.StartDate.Format("2006-01-02"), e.EndDate.Format("2006-01-02"))
}

func (e *InvalidDateRangeError) Unwrap() error {
	return ErrInvalidDateRange
}

// BatchOperationError wraps errors that occur during batch operations
type BatchOperationError struct {
	Errors []error
}

func (e *BatchOperationError) Error() string {
	return fmt.Sprintf("batch operation failed with %d errors", len(e.Errors))
}

func (e *BatchOperationError) AddError(err error) {
	e.Errors = append(e.Errors, err)
}

func (e *BatchOperationError) HasErrors() bool {
	return len(e.Errors) > 0
}
