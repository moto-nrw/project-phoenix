// Package transactionruntime exposes database failure classification without
// exposing a driver or database handle to HTTP consumers.
package transactionruntime

import "github.com/moto-nrw/project-phoenix/modules/transactionruntime/internal/adapters/postgres"

func IsTransientDatabaseError(err error) bool { return postgres.IsTransientDatabaseError(err) }
func IsConstraintViolation(err error) bool    { return postgres.IsConstraintViolation(err) }

// ErrNoRows is the sentinel a retained repository returns or wraps when no
// row matched. Callers classify with errors.Is and may wrap it themselves.
var ErrNoRows = postgres.ErrNoRows
