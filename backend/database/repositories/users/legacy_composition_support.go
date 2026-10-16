package users

import (
	"database/sql"
	"errors"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The helpers below serve the legacy composition seam
// (database/repositories), which assembles the staff, teacher and guest
// repositories over the School Membership owner and reports their failures in
// the repository error shape these repositories use. The caller's school
// comes from peopledirectory/compose and the permission matcher from
// modules/securityruntime (#2727).

// NotFoundError builds the not-found shape every legacy repository returns, so
// callers keep classifying with errors.Is(err, sql.ErrNoRows) and
// IsRepositoryNoRows(err) alike.
func NotFoundError(op string) error {
	return &DatabaseError{Op: op, Err: translateNotFound(sql.ErrNoRows)}
}

// MissingStudentError is the not-found shape of a child that is not there,
// with the no-rows sentinel the retained callers match (#3349).
func MissingStudentError(op string) error {
	return &DatabaseError{Op: op, Err: errors.Join(ErrRepositoryNotFound, sql.ErrNoRows, usersModels.ErrStudentRowMissing)}
}

// WrapError wraps err in the legacy repository error shape, preserving the
// chain so errors.Is on the original sentinel still works.
func WrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &DatabaseError{Op: op, Err: err}
}

// ParseCalendarDate parses a YYYY-MM-DD value; an empty or malformed value
// yields nil, which is how the legacy models spell "no date".
func ParseCalendarDate(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}
