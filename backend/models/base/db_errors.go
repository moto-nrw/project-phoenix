package base

import (
	"errors"
	"strings"
)

// ErrNotFound is the persistence-neutral repository result for a missing row.
var ErrNotFound = errors.New("repository: not found")

type postgresError interface {
	error
	Field(byte) string
}

// IsUniqueViolation reports whether err carries PostgreSQL error code 23505
// (unique_violation). It prefers structured unwrapping through DatabaseError
// and other wrappers, then recognizes the driver's SQLSTATE text when an error
// boundary has discarded the concrete pgdriver type.
func IsUniqueViolation(err error) bool {
	var pgErr postgresError
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == "23505"
	}
	return hasTextualSQLState(err, "23505")
}

// IsUniqueViolationOn is IsUniqueViolation restricted to one constraint or
// index name (pgdriver message field 'n', or the degraded error text).
func IsUniqueViolationOn(err error, constraint string) bool {
	if constraint == "" {
		return false
	}

	var pgErr postgresError
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == "23505" &&
			(pgErr.Field('n') == constraint || hasTextualConstraint(pgErr.Error(), constraint))
	}
	return hasTextualSQLState(err, "23505") && hasTextualConstraint(err.Error(), constraint)
}

func hasTextualSQLState(err error, code string) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE="+code)
}

func hasTextualConstraint(message, identifier string) bool {
	primary, _, _ := strings.Cut(message, "\n")
	primary, _, found := strings.Cut(primary, " (SQLSTATE=")
	if !found {
		return false
	}

	opening := strings.LastIndex(primary, "»")
	if opening >= 0 && strings.HasSuffix(primary, "«") {
		return primary[opening+len("»"):len(primary)-len("«")] == identifier
	}

	quoted, found := lastDoubleQuotedIdentifier(primary)
	return found && quoted == identifier
}

func lastDoubleQuotedIdentifier(message string) (string, bool) {
	var last string
	var found bool

	for offset := 0; offset < len(message); offset++ {
		if message[offset] != '"' {
			continue
		}

		var quoted strings.Builder
		for offset++; offset < len(message); offset++ {
			if message[offset] != '"' {
				quoted.WriteByte(message[offset])
				continue
			}
			if offset+1 < len(message) && message[offset+1] == '"' {
				quoted.WriteByte('"')
				offset++
				continue
			}
			last = quoted.String()
			found = true
			break
		}
	}
	return last, found
}

// IsLockNotAvailable reports whether err carries PostgreSQL error code 55P03
// (lock_not_available) — what `SELECT … FOR UPDATE NOWAIT` raises when the row
// is already locked by another transaction. Callers use it to turn a refused
// out-of-order lock into a retriable conflict instead of blocking (which would
// risk a deadlock) or failing as a 500.
func IsLockNotAvailable(err error) bool {
	var pgErr postgresError
	return errors.As(err, &pgErr) && pgErr.Field('C') == "55P03"
}

// IsConstraintViolation reports whether err carries PostgreSQL error code
// 23503 (foreign_key_violation) or 23502 (not_null_violation): a delete or
// write refused because another row still references it or a required
// column was left empty.
func IsConstraintViolation(err error) bool {
	var pgErr postgresError
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		return code == "23503" || code == "23502"
	}
	return hasTextualSQLState(err, "23503") || hasTextualSQLState(err, "23502")
}

// IsNoRows reports whether err wraps the persistence-neutral repository
// not-found sentinel.
func IsNoRows(err error) bool {
	return errors.Is(err, ErrNotFound)
}
