// Package transactionruntime exposes database failure classification without
// exposing a driver or database handle to HTTP consumers.
package transactionruntime

import "github.com/moto-nrw/project-phoenix/modules/transactionruntime/internal/adapters/postgres"

func IsTransientDatabaseError(err error) bool { return postgres.IsTransientDatabaseError(err) }
func IsConstraintViolation(err error) bool    { return postgres.IsConstraintViolation(err) }

// IsUniqueViolation reports PostgreSQL's unique_violation; IsUniqueViolationOn
// restricts it to one constraint or index name.
func IsUniqueViolation(err error) bool { return postgres.IsUniqueViolation(err) }
func IsUniqueViolationOn(err error, constraint string) bool {
	return postgres.IsUniqueViolationOn(err, constraint)
}

// ErrNoRows is the sentinel a retained repository returns or wraps when no
// row matched. Callers classify with errors.Is and may wrap it themselves.
var ErrNoRows = postgres.ErrNoRows

// RecordErrors builds the retained repository error shape for the root's
// adapters over owner records: a missing row is both a repository and an SQL
// not-found error, and every failure is a store failure.
type RecordErrors struct{}

// WrapRecordError wraps err in the retained repository error shape.
func (RecordErrors) WrapRecordError(operation string, err error) error {
	return postgres.WrapRecordError(operation, err)
}

// MissingRecordError is the retained not-found error of operation.
func (RecordErrors) MissingRecordError(operation string) error {
	return postgres.MissingRecordError(operation)
}
