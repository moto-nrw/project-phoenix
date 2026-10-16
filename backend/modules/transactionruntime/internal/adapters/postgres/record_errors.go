package postgres

import (
	"database/sql"
	"errors"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// WrapRecordError wraps err of operation in the retained repository error
// shape, the DatabaseError the retained callers of other owners' records
// classify on; nil stays nil.
func WrapRecordError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &modelBase.DatabaseError{Op: operation, Err: err}
}

// MissingRecordError is the retained not-found error of operation: both the
// repository not-found sentinel and the driver's no-rows sentinel.
func MissingRecordError(operation string) error {
	return WrapRecordError(operation, errors.Join(modelBase.ErrNotFound, sql.ErrNoRows))
}
