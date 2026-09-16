package auth

import (
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

// IsNotFound reports whether err is the retained repositories' missing-row
// outcome, as TranslateNotFound records it.
func IsNotFound(err error) bool {
	return errors.Is(err, modelBase.ErrNotFound)
}
