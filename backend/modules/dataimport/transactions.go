package dataimport

import (
	"context"
	"errors"
	"time"
)

// TransactionObservation contains only the timing and retry facts imports use.
type TransactionObservation struct {
	PoolWait time.Duration
	LockWait time.Duration
	Retries  int
}

// Transactions is the import workflow's tenant execution port. Implementations
// must preserve the caller's tenant and transaction context across callbacks.
// Savepoint runs fn directly, without a savepoint, when the context carries
// the row-write mark of WithBatchRowWrite; it hands fn a context without the
// mark, so savepoints fn opens itself stay real.
type Transactions interface {
	TenantID(context.Context) int64
	HasTransaction(context.Context) bool
	WithinCurrent(context.Context, func(context.Context) error) error
	WithinRetry(context.Context, func(context.Context) error) error
	Savepoint(context.Context, func(context.Context) error) error
	AcquireLock(context.Context, string) error
	Observe(context.Context, func(TransactionObservation)) context.Context
}

// ErrSavepointControl marks a failure to open, roll back or release a
// savepoint. The transaction is no longer usable, so the chunk executor
// returns it instead of replaying the chunk.
var ErrSavepointControl = errors.New("data import: savepoint control failed")

// batchRowWriteKey marks one row's owner writes in ImportBatches. There a
// failed row write aborts the whole chunk, so the savepoint every config opens
// around its row protects no other row and only costs two statements (#5552,
// ADR 0067 addendum of 2026-11-08, step 3). The legacy Import() sets no mark
// and keeps the savepoint: it records the failed row and goes on.
type batchRowWriteKey struct{}

// WithBatchRowWrite marks ctx for the next Savepoint of one row write.
func WithBatchRowWrite(ctx context.Context) context.Context {
	return context.WithValue(ctx, batchRowWriteKey{}, true)
}

// TakeBatchRowWrite reports the mark and returns ctx without it.
func TakeBatchRowWrite(ctx context.Context) (context.Context, bool) {
	if marked, _ := ctx.Value(batchRowWriteKey{}).(bool); !marked {
		return ctx, false
	}
	return context.WithValue(ctx, batchRowWriteKey{}, false), true
}
