package runtimebinding

import (
	"context"
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
func ImportSavepoint(ctx context.Context, fn func(context.Context) error, savepoint func(context.Context, func(context.Context) error) error) error {
	if ctx, rowWrite := dataimport.TakeBatchRowWrite(ctx); rowWrite {
		return fn(ctx)
	}
	return savepoint(ctx, fn)
}
