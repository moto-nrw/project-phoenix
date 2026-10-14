package api

import (
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The staff, teacher, guest and teaching-assignment adapters report their
// failures in the retained repository error shape (models/users), so callers
// keep classifying with errors.Is(err, sql.ErrNoRows) and
// userModels.IsRepositoryNoRows(err) alike. They moved here from the
// repository composition with the adapters (#2743).

// repositoryNotFound builds the not-found shape every retained repository
// returns.
func repositoryNotFound(op string) error {
	return &userModels.DatabaseError{Op: op, Err: errors.Join(userModels.ErrRepositoryNotFound, transactionruntime.ErrNoRows)}
}

// repositoryError wraps err in the retained repository error shape,
// preserving the chain so errors.Is on the original sentinel still works.
func repositoryError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &userModels.DatabaseError{Op: op, Err: err}
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
