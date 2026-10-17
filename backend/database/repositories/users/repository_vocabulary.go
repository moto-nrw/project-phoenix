package users

import (
	"errors"
	"fmt"
	"strings"

	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The query and error vocabulary of the retained People Directory
// repositories. It moved here from models/users, next to the repositories
// that read it, so the row package keeps only the rows (#3990). The shapes
// match the generic repository vocabulary of models/base without naming it:
// callers outside People Directory recognise the not-found sentinel and the
// store failure by their marker methods, not by identity.

type postgresError interface {
	error
	Field(byte) string
}

// guardianEmailIndex is the per-school, case-insensitive unique index on
// users.guardian_profiles (tenant_id, LOWER(email)).
const guardianEmailIndex = "idx_guardian_profiles_tenant_email"

// guardianWriteError marks the e-mail index refusal of a guardian profile
// write with ErrGuardianEmailTaken and keeps the driver error in the chain.
func guardianWriteError(message string, err error) error {
	var pgErr postgresError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Field('C'), "23") && pgErr.Field('n') == guardianEmailIndex {
		return fmt.Errorf("%s: %w: %w", message, ports.ErrGuardianEmailTaken, err)
	}
	return fmt.Errorf("%s: %w", message, err)
}

// IsConstraintViolation reports whether err carries PostgreSQL error code
// 23503 (foreign_key_violation) or 23502 (not_null_violation), such as the
// refusal of a guardian delete that still has links.
func IsConstraintViolation(err error) bool {
	var pgErr postgresError
	if errors.As(err, &pgErr) {
		code := pgErr.Field('C')
		return code == "23503" || code == "23502"
	}
	return hasTextualSQLState(err, "23503") || hasTextualSQLState(err, "23502")
}

func hasTextualSQLState(err error, sqlState string) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE="+sqlState)
}
