package owners

import (
	"errors"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// repositoryNotFound builds the not-found shape every retained repository
// returns.
func repositoryNotFound(op string) error {
	return &peoplePorts.DatabaseError{Op: op, Err: errors.Join(peoplePorts.ErrRepositoryNotFound, transactionruntime.ErrNoRows)}
}

// repositoryError wraps err in the retained repository error shape,
// preserving the chain so errors.Is on the original sentinel still works.
func repositoryError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &peoplePorts.DatabaseError{Op: op, Err: err}
}

// calendarDateString renders an optional calendar date as YYYY-MM-DD, empty
// when unset.
func calendarDateString(value *calendar.Date) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.String()
}

// parseCalendarDate parses a YYYY-MM-DD value; an empty or malformed value
// yields nil, which is how the retained models spell "no date".
func parseCalendarDate(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}
