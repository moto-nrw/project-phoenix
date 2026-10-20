package timetracking

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// recordErrors builds the retained repository error shape of the capability
// row adapters through the transaction runtime: every failure wraps its
// operation, a missing row is both a repository and an SQL not-found error.
var recordErrors transactionruntime.RecordErrors

// rowsAffectedError is the legacy repository result for an update that matched no
// row: the same error the generic repository produced.
func rowsAffectedError(op string) error {
	return recordErrors.WrapRecordError(op, fmt.Errorf("expected %d rows affected, got %d", 1, 0))
}

// rowReadError restores the legacy read error shape: a missing row is the
// retained not-found error, every other failure keeps its cause behind the
// operation.
func rowReadError(op string, err error, notFound error) error {
	if notFound != nil && errors.Is(err, notFound) {
		return recordErrors.MissingRecordError(op)
	}
	return recordErrors.WrapRecordError(op, err)
}
