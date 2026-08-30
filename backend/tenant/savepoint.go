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
		return fmt.Errorf("%w: create: %v", ErrSavepointControl, err)
	}

	hooks, hookCount := afterCommitCheckpoint(ctx)
	operationErr := fn(ctx)
	if operationErr != nil {
		if err := uow.savepoint(ctx, RollbackSavepoint); err != nil {
			return errors.Join(operationErr, fmt.Errorf("%w: rollback: %v", ErrSavepointControl, err))
		}
		discardAfterCommitHooksAfter(hooks, hookCount)
		if err := uow.savepoint(ctx, ReleaseSavepoint); err != nil {
			return errors.Join(operationErr, fmt.Errorf("%w: release after rollback: %v", ErrSavepointControl, err))
		}
		return operationErr
	}

	if err := uow.savepoint(ctx, ReleaseSavepoint); err != nil {
		return fmt.Errorf("%w: release: %v", ErrSavepointControl, err)
	}
	return nil
}

func afterCommitCheckpoint(ctx context.Context) (*afterCommitHooks, int) {
	hooks, _ := ctx.Value(afterCommitKey{}).(*afterCommitHooks)
	if hooks == nil {
		return nil, 0
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	return hooks, len(hooks.fns)
}

func discardAfterCommitHooksAfter(hooks *afterCommitHooks, count int) {
	if hooks == nil {
		return
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	if count < 0 {
		count = 0
	}
	if count < len(hooks.fns) {
		hooks.fns = hooks.fns[:count]
	}
}
