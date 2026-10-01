package users

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// RowMissing reports the "no row" outcomes the retained repositories use: the
// translated not-found sentinel and the guardian sentinels this package owns.
// Those repositories translate a driver no-rows result into one of these
// before it gets here. The composition root classifies repository results
// with it when it binds the Identity & Access seams (#3364).
func RowMissing(err error) bool {
	return errors.Is(err, base.ErrNotFound) ||
		errors.Is(err, ErrGuardianProfileNotFound) || errors.Is(err, ErrStudentGuardianNotFound)
}
