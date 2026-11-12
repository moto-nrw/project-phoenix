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

// IsSavepointControlFailure reports a savepoint-control failure anywhere in
// err, nested savepoints included: the transaction err comes from can no
// longer be trusted.
func IsSavepointControlFailure(err error) bool { return errors.Is(err, ErrSavepointControl) }

// ErrSavepointRollback classifies a savepoint whose rollback failed: ROLLBACK
// TO SAVEPOINT itself, or the RELEASE after it. Such a failure also wraps
// ErrSavepointControl. The transaction's state is unknown, so the unit is
// never replayed (E19, #6316). A failure to create or to release a savepoint
// after success is a savepoint-control failure the runtime may still replay
// as a whole.
var ErrSavepointRollback = errors.New("savepoint rollback failed")

// SavepointRollbackFailure marks err, a savepoint-control failure of a
// rollback, with ErrSavepointRollback. Its text stays err's text, and
// database classifiers skip it as attached diagnostics.
func SavepointRollbackFailure(err error) error { return savepointRollbackFailure{err} }

type savepointRollbackFailure struct{ error }

func (f savepointRollbackFailure) Unwrap() error      { return f.error }
func (savepointRollbackFailure) Is(target error) bool { return target == ErrSavepointRollback }

// CleanupDiagnostic marks the failure as diagnostics attached to the
// operation's error: database classifiers (retry, connectivity, constraint,
// unique) read the operation's own result only (#6316).
func (savepointRollbackFailure) CleanupDiagnostic() {}

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

	hooks, commitHookCount := afterCommitCheckpoint(ctx)
	restoreLocks := lockCheckpoint(ctx)
	operationErr := fn(ctx)
	if operationErr != nil {
		if err := uow.savepoint(ctx, RollbackSavepoint); err != nil {
			return errors.Join(operationErr, SavepointRollbackFailure(fmt.Errorf("%w: rollback: %w", ErrSavepointControl, err)))
		}
		discardSavepointCommitHooks(hooks, commitHookCount)
		restoreLocks()
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

func afterCommitCheckpoint(ctx context.Context) (*afterCommitHooks, int) {
	hooks, _ := ctx.Value(afterCommitKey{}).(*afterCommitHooks)
	if hooks == nil {
		return nil, 0
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()
	return hooks, len(hooks.fns)
}

func discardSavepointCommitHooks(hooks *afterCommitHooks, commitCount int) {
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
	hooks.mu.Unlock()
}
