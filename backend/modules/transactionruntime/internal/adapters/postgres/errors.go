package postgres

import (
	"context"
	"database/sql/driver"
	"net"
	"strings"

	"github.com/uptrace/bun/driver/pgdriver"
)

// IsTransientDatabaseError reports whether err represents a temporary database
// connectivity failure rather than a domain validation error. Callers can use
// this to retry a whole transaction once or return 503 after retry exhaustion.
func IsTransientDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	if operationIs(err, context.Canceled) || operationIs(err, context.DeadlineExceeded) {
		return false
	}
	if operationIs(err, driver.ErrBadConn) {
		return true
	}

	if _, ok := operationAs[net.Error](err); ok {
		return true
	}

	if pgErr, ok := operationAs[pgdriver.Error](err); ok {
		code := pgErr.Field('C')
		return len(code) >= 2 && code[:2] == "08"
	}

	return strings.Contains(operationText(err), "driver: bad connection")
}

// IsConstraintViolation checks if an error is a PostgreSQL constraint violation
// that indicates the entity cannot be deleted due to dependencies.
// Primary check uses typed pgdriver.Error with SQLSTATE codes (23503 = FK, 23502 = NOT NULL).
// Fallback string matching covers errors that have been wrapped and lost the original type.
func IsConstraintViolation(err error) bool {
	if err == nil {
		return false
	}

	// Primary: typed pgdriver.Error with structured SQLSTATE code
	if pgErr, ok := operationAs[pgdriver.Error](err); ok {
		code := pgErr.Field('C') // SQLSTATE code
		return code == "23503" || code == "23502"
	}

	// Fallback: string matching for wrapped errors that lost the pgdriver.Error type
	msg := operationText(err)
	return strings.Contains(msg, "violates foreign key constraint") ||
		strings.Contains(msg, "violates not-null constraint")
}
