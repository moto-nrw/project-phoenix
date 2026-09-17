package auth

import (
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// isNotFoundError reports the "no row" outcomes of the retained
// repositories the invitation and guardian flows still read through.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, userModels.ErrGuardianProfileNotFound) {
		return true
	}
	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}
