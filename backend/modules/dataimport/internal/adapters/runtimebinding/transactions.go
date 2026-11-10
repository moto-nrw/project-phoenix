package runtimebinding

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

func ImportTransactionPresent(ctx context.Context, current func(context.Context) (any, bool)) bool {
	_, ok := current(ctx)
	return ok
}
func RetryImportTransaction[I any](ctx context.Context, fn func(context.Context) error, school func(context.Context) (I, error), execute func(context.Context, I, func(context.Context) error) error) error {
	id, err := school(ctx)
	if err != nil {
		return err
	}
	return execute(ctx, id, fn)
}
func ObserveImportTransaction[K comparable](observe func(dataimport.TransactionObservation), kind K, duration time.Duration, retries int, pool K, lock K, transaction K) {
	var observation dataimport.TransactionObservation
	switch kind {
	case pool:
		observation.PoolWait = duration
	case lock:
		observation.LockWait = duration
	case transaction:
		observation.Retries = retries
	default:
		return
	}
	observe(observation)
}

// ImportSavepoint runs the first savepoint of a batch row write directly and
// every other one through savepoint (dataimport.WithBatchRowWrite, #5552).
// A failure of this savepoint's own statements carries
// dataimport.ErrSavepointControl. It is told apart by what savepoint returns,
// not by searching the error: fn's failure reaches savepoint as a marker of
// this call, and savepoint returns that very marker only when its own
// statements succeeded. A failed savepoint nested in fn therefore stays fn's
// error (#5552 review of 1bb4bde258).
func ImportSavepoint(ctx context.Context, fn func(context.Context) error, savepoint func(context.Context, func(context.Context) error) error) error {
	if ctx, rowWrite := dataimport.TakeBatchRowWrite(ctx); rowWrite {
		return fn(ctx)
	}
	failed := errors.New("data import: savepoint operation failed")
	var operationErr error
	err := savepoint(ctx, func(ctx context.Context) error {
		if operationErr = fn(ctx); operationErr != nil {
			return failed
		}
		return nil
	})
	switch {
	case err == nil:
		return nil
	case err == failed: //nolint:errorlint // identity: the savepoint's own statements succeeded
		return operationErr
	case operationErr != nil:
		return errors.Join(operationErr, fmt.Errorf("%w: %w", dataimport.ErrSavepointControl, err))
	default:
		return fmt.Errorf("%w: %w", dataimport.ErrSavepointControl, err)
	}
}
