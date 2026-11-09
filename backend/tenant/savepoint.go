package tenant

import (
	"context"
	"errors"
	"fmt"
)

// ErrSavepointControl classifies failures to create, roll back, or release a
// savepoint. Callers that otherwise continue after an operation error must
// abort the surrounding transaction when this sentinel is present: its state
// can no longer be trusted.
var ErrSavepointControl = errors.New("savepoint control failed")

// ErrSavepointRollback classifies a savepoint whose rollback failed: ROLLBACK
// TO SAVEPOINT itself, or the RELEASE after it. Such a failure also wraps
// ErrSavepointControl. The transaction's state is unknown, so the unit is
// never replayed (E19, #6316). A failure to create or to release a savepoint
// after success is a savepoint-control failure the runtime may still replay
// as a whole.
var ErrSavepointRollback = errors.New("savepoint rollback failed")

// SavepointRollbackFailure marks err, a savepoint-control failure of a
// rollback, with ErrSavepointRollback. Its text stays err's text.
func SavepointRollbackFailure(err error) error { return savepointRollbackFailure{err} }

type savepointRollbackFailure struct{ error }

func (f savepointRollbackFailure) Unwrap() error      { return f.error }
func (savepointRollbackFailure) Is(target error) bool { return target == ErrSavepointRollback }

// operationSavepointName is internal and fixed, so no caller-controlled SQL
// identifier is interpolated. PostgreSQL permits nested savepoints with the
// same name; RELEASE and ROLLBACK TO address the most recently established
// one, which gives nested WithSavepoint calls the expected stack semantics.
// WithSavepoint runs fn inside a PostgreSQL savepoint on the transaction in
// ctx. A normal fn error is returned after its writes and after-commit hooks
// have been rolled back, leaving the surrounding transaction usable. Any
// savepoint-control failure wraps ErrSavepointControl and must be treated as
// fatal by the caller.
//
// The function is intentionally strict: callers must already own a
// transaction. This avoids pretending that a direct, non-transactional call
// has row-level atomicity.
func WithSavepoint(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return fmt.Errorf("%w: callback is required", ErrSavepointControl)
	}

	uow, err := unitOfWorkFromContext(ctx)
	if err != nil {
		return fmt.Errorf("%w: transaction is required", ErrSavepointControl)
	}

	if err := uow.savepoint(ctx, CreateSavepoint); err != nil {
		return fmt.Errorf("%w: create: %w", ErrSavepointControl, err)
	}

	hooks, commitHookCount, rollbackHookCount := transactionHookCheckpoint(ctx)
	operationErr := fn(ctx)
	if operationErr != nil {
		if err := uow.savepoint(ctx, RollbackSavepoint); err != nil {
			return errors.Join(operationErr, SavepointRollbackFailure(fmt.Errorf("%w: rollback: %w", ErrSavepointControl, err)))
		}
		runSavepointRollbackHooks(hooks, commitHookCount, rollbackHookCount)
		if err := uow.savepoint(ctx, ReleaseSavepoint); err != nil {
			return errors.Join(operationErr, SavepointRollbackFailure(fmt.Errorf("%w: release after rollback: %w", ErrSavepointControl, err)))
		}
		return operationErr
	}

	if err := uow.savepoint(ctx, ReleaseSavepoint); err != nil {
		return fmt.Errorf("%w: release: %w", ErrSavepointControl, err)
	}
	return nil
}

func transactionHookCheckpoint(ctx context.Context) (*afterCommitHooks, int, int) {
	hooks, _ := ctx.Value(afterCommitKey{}).(*afterCommitHooks)
	if hooks == nil {
		return nil, 0, 0
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	return hooks, len(hooks.fns), len(hooks.rollbackFns)
}

func runSavepointRollbackHooks(hooks *afterCommitHooks, commitCount, rollbackCount int) {
	if hooks == nil {
		return
	}
	hooks.mu.Lock()
	if commitCount < 0 {
		commitCount = 0
	}
	if commitCount > len(hooks.fns) {
		commitCount = len(hooks.fns)
	}
	if commitCount < len(hooks.fns) {
		hooks.fns = hooks.fns[:commitCount]
	}
	if rollbackCount < 0 {
		rollbackCount = 0
	}
	if rollbackCount > len(hooks.rollbackFns) {
		rollbackCount = len(hooks.rollbackFns)
	}
	fns := append([]func(){}, hooks.rollbackFns[rollbackCount:]...)
	hooks.rollbackFns = hooks.rollbackFns[:rollbackCount]
	hooks.mu.Unlock()
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}
