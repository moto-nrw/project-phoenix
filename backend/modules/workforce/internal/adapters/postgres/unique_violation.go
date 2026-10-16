package postgres

import (
	"errors"

	"github.com/uptrace/bun/driver/pgdriver"
)

// sqlStateUniqueViolation is PostgreSQL's unique_violation.
const sqlStateUniqueViolation = "23505"

// isUniqueViolation reports whether err carries PostgreSQL's unique_violation.
func isUniqueViolation(err error) bool {
	pgErr, ok := errors.AsType[pgdriver.Error](err)
	return ok && pgErr.Field('C') == sqlStateUniqueViolation
}

// isUniqueViolationOn is isUniqueViolation restricted to one constraint or
// unique index name (the driver's message field 'n').
func isUniqueViolationOn(err error, constraint string) bool {
	pgErr, ok := errors.AsType[pgdriver.Error](err)
	return ok && constraint != "" && pgErr.Field('C') == sqlStateUniqueViolation && pgErr.Field('n') == constraint
}
