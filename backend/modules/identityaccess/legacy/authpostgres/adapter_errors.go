package authpostgres

import (
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// DatabaseError wraps a persistence failure in the retained repository error
// contract. The compatibility adapters the composition root builds over the
// Identity & Access owner (#2720) use it so their callers keep seeing the
// same error shape the retained repositories produced.
func DatabaseError(op string, err error) error {
	return &modelBase.DatabaseError{Op: op, Err: err}
}

// IsMissingRow reports whether err is a missing row in either spelling the
// retained repositories produce: the base not-found sentinel that
// TranslateNotFound records, and the driver's own sql.ErrNoRows, which the
// lookups that scan a single row return unwrapped.
func IsMissingRow(err error) bool {
	return errors.Is(err, modelBase.ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}
