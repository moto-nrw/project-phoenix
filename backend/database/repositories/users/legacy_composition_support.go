package users

import (
	"database/sql"

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
// usersModels.IsRepositoryNoRows(err) alike.
func NotFoundError(op string) error {
	return &usersModels.DatabaseError{Op: op, Err: translateNotFound(sql.ErrNoRows)}
}

// WrapError wraps err in the legacy repository error shape, preserving the
// chain so errors.Is on the original sentinel still works.
func WrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &usersModels.DatabaseError{Op: op, Err: err}
}

// IsNotFound reports whether err is a repository not-found error.
func IsNotFound(err error) bool { return usersModels.IsRepositoryNoRows(err) }

// CalendarDateString renders an optional calendar date as YYYY-MM-DD, empty
// when unset.
func CalendarDateString(value *calendar.Date) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.String()
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

// TodayCalendarDate is today in Berlin as YYYY-MM-DD.
func TodayCalendarDate() string { return calendar.TodayDate().String() }

// IsUniqueViolation reports whether err is a PostgreSQL unique violation.
func IsUniqueViolation(err error) bool { return usersModels.IsUniqueViolation(err) }

// IsUniqueViolationOn reports whether err is a unique violation of the named
// constraint or index.
func IsUniqueViolationOn(err error, name string) bool {
	return usersModels.IsUniqueViolationOn(err, name)
}
