package postgres

import (
	"context"
	"database/sql"
	"errors"
	"net"

	"github.com/uptrace/bun/driver/pgdriver"
)

// SchoolReadErrors classifies the retained school read for Root-bound consumers.
type SchoolReadErrors struct{}

// Missing reports the existing SQL not-found outcome, including wrapped errors.
func (SchoolReadErrors) Missing(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// Transient returns true for errors that indicate a temporary
// connectivity problem (net timeouts, connection resets, context
// cancellation, PostgreSQL connection-class SQLSTATE 08xxx). Everything else
// (bad queries, permission errors, serialization failures) returns false so
// the adapter fails closed. Wrapped repository errors are unwrapped through
// the standard error chain.
func (SchoolReadErrors) Transient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		return len(code) >= 2 && code[:2] == "08"
	}
	return false
}
