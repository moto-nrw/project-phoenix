package dataimport

import (
	"context"
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
type Transactions interface {
	TenantID(context.Context) int64
	HasTransaction(context.Context) bool
	WithinCurrent(context.Context, func(context.Context) error) error
	WithinRetry(context.Context, func(context.Context) error) error
	Savepoint(context.Context, func(context.Context) error) error
	AcquireLock(context.Context, string) error
	Observe(context.Context, func(TransactionObservation)) context.Context
}
