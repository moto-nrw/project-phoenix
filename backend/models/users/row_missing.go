package users

import (
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RowMissing reports the "no row" outcomes the retained repositories use: a
// bare or wrapped sql.ErrNoRows, the translated not-found sentinel and the
// guardian sentinels this package owns. The composition root classifies
// repository results with it when it binds the Identity & Access seams
// (#3364).
func RowMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, base.ErrNotFound) ||
		errors.Is(err, ErrGuardianProfileNotFound) || errors.Is(err, ErrStudentGuardianNotFound) {
		return true
	}
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}
	return false
}
