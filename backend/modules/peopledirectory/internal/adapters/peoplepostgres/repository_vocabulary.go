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
