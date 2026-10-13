package compose

import (
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RetainedRecordErrors keeps the retained People Directory repository error
// shape for the legacy callers of another owner's records (the Student
// Presence session records): a missing row stays both a repository and an
// SQL not-found error.
type RetainedRecordErrors struct{}

// WrapRecordError wraps err in the retained repository error shape.
func (RetainedRecordErrors) WrapRecordError(operation string, err error) error {
	return usersRepo.WrapError(operation, err)
}

// MissingRecordError is the retained not-found error of operation.
func (RetainedRecordErrors) MissingRecordError(operation string) error {
	return usersRepo.NotFoundError(operation)
}

// ParseRetainedCalendarDate parses a YYYY-MM-DD value the way the retained
// repositories do: an empty or malformed value is no date.
func ParseRetainedCalendarDate(value string) *calendar.Date {
	return usersRepo.ParseCalendarDate(value)
}
