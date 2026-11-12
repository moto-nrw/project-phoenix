package tenant

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrLockWaitExceeded reports a lock wait that WithLockTimeout ended: another
// transaction held the lock longer than the bound.
var ErrLockWaitExceeded = errors.New("tenant: lock wait exceeded its bound")

// WithLockBound installs the adapter function that bounds the lock waits of
// a callback in the current transaction (WithLockTimeout).
func (uow UnitOfWork) WithLockBound(bound func(context.Context, time.Duration, func(context.Context) error) error) UnitOfWork {
	uow.boundLocks = bound
	return uow
}

// WithLockTimeout runs fn with PostgreSQL's lock_timeout set to limit for the
// current transaction only, and restores the previous value when fn
// succeeds. A lock wait of fn longer than limit fails with an error wrapping
// ErrLockWaitExceeded (55P03); the transaction is aborted then and the
// caller rolls it back, or the savepoint enclosing the call, which also
// undoes the setting. Unlike a statement timeout the connection stays open,
// and statements that wait for no lock run unbounded. It serves a unit that
// gives way to a lock holder and runs later instead of holding up the units
// after it (#6703).
func WithLockTimeout(ctx context.Context, limit time.Duration, fn func(context.Context) error) error {
	if limit <= 0 {
		return fmt.Errorf("tenant: lock timeout must be positive, got %s", limit)
	}
	uow, err := unitOfWorkFromContext(ctx)
	if err != nil {
		return err
	}
	if uow.boundLocks == nil {
		return fmt.Errorf("%w: lock bound function is required", ErrRuntimeRequired)
	}
	err = uow.boundLocks(ctx, limit, fn)
	var exceeded interface{ LockWaitExceeded() }
	if errors.As(err, &exceeded) {
		return fmt.Errorf("%w: %w", ErrLockWaitExceeded, err)
	}
	return err
}
