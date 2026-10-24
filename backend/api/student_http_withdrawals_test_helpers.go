//go:build testhooks

package api

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// StudentHTTPTestRepositoryNotFound is the retained repositories' not-found
// sentinel (models/base.ErrNotFound), for suites that pin api/common's
// classifier against it. Transaction Runtime's missing-record error joins it
// with sql.ErrNoRows; the sentinel is the join's first error.
var StudentHTTPTestRepositoryNotFound = func() error {
	var joined interface{ Unwrap() []error }
	if !errors.As(transactionruntime.RecordErrors{}.MissingRecordError("lookup"), &joined) || len(joined.Unwrap()) == 0 {
		panic("transaction runtime: the missing-record error holds no not-found sentinel")
	}
	return joined.Unwrap()[0]
}()
