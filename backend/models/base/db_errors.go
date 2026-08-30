package base

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/uptrace/bun/driver/pgdriver"
)

// IsUniqueViolation reports whether err carries PostgreSQL error code 23505
// (unique_violation). It prefers structured unwrapping through DatabaseError
// and other wrappers, then recognizes the driver's SQLSTATE text when an error
// boundary has discarded the concrete pgdriver type.
func IsUniqueViolation(err error) bool {
	var pgErr pgdriver.Error
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

	var pgErr pgdriver.Error
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
	return hasDoubleQuotedIdentifier(primary, identifier) ||
		strings.Contains(primary, "»"+identifier+"«")
}

func hasDoubleQuotedIdentifier(message, identifier string) bool {
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
			if quoted.String() == identifier {
				return true
			}
			break
		}
	}
	return false
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
