package postgres

import (
	"errors"
	"strings"
)

// postgresError is the message-field accessor both the pgdriver error and
// its degraded forms expose; matching on it keeps the SQLSTATE handling
// inside the adapter that produced it.
type postgresError interface {
	error
	Field(name byte) string
}

// isUniqueViolation reports a PostgreSQL unique-constraint failure (23505).
// The textual fallback covers the errors a wrapped or joined chain degrades
// to, where only the message survives.
func isUniqueViolation(err error) bool {
	var pgErr postgresError
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == "23505"
	}
	return err != nil && strings.Contains(err.Error(), "SQLSTATE=23505")
}
