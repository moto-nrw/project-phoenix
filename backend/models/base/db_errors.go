package base

import (
	"database/sql"
	"errors"

	"github.com/uptrace/bun/driver/pgdriver"
)

// IsUniqueViolation reports whether err carries PostgreSQL error code 23505
// (unique_violation). errors.As unwraps DatabaseError and other wrappers down
// to the driver error, same as IsRetryableTxError.
func IsUniqueViolation(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.Field('C') == "23505"
}

// IsUniqueViolationOn is IsUniqueViolation restricted to one constraint or
// index name (pgdriver message field 'n').
func IsUniqueViolationOn(err error, constraint string) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) &&
		pgErr.Field('C') == "23505" &&
		pgErr.Field('n') == constraint
}

// IsLockNotAvailable reports whether err carries PostgreSQL error code 55P03
// (lock_not_available) — what `SELECT … FOR UPDATE NOWAIT` raises when the row
// is already locked by another transaction. Callers use it to turn a refused
// out-of-order lock into a retriable conflict instead of blocking (which would
// risk a deadlock) or failing as a 500.
func IsLockNotAvailable(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.Field('C') == "55P03"
}

// IsNoRows reports whether err is (or wraps, incl. via DatabaseError)
// sql.ErrNoRows — the canonical "not found" classification for repo results.
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
