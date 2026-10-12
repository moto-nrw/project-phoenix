// Package transactionruntime exposes database failure classification without
// exposing a driver or database handle to HTTP consumers.
package transactionruntime

import "github.com/moto-nrw/project-phoenix/modules/transactionruntime/internal/adapters/postgres"

func IsTransientDatabaseError(err error) bool { return postgres.IsTransientDatabaseError(err) }
func IsConstraintViolation(err error) bool    { return postgres.IsConstraintViolation(err) }
