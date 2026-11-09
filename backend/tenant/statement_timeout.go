package tenant

import (
	"context"
	"fmt"
	"time"
)

// WithStatementBound installs the adapter function that bounds the statements
// of a callback in the current transaction (WithStatementTimeout).
func (uow UnitOfWork) WithStatementBound(bound func(context.Context, time.Duration, func(context.Context) error) error) UnitOfWork {
	uow.boundStatements = bound
	return uow
}

// WithStatementTimeout runs fn with PostgreSQL's statement_timeout set to
// limit for the current transaction only, and restores the previous value
// when fn succeeds. It serves a bounded attempt (decision E13 of 09.11.2026,
// ADR 0067 addendum of 2026-11-08): a bundled write must not wait longer than
// limit in one read, so its waits cannot add up past the driver's read
// timeout. When fn fails, the limit stays set until the caller rolls back the
// savepoint that encloses the call, which also undoes the setting. A
// statement the limit cancels (57014) ends the transaction instead: pgdriver
// treats 57014 as a broken connection and closes it (pgdriver v1.3.0,
// error.go:55–82), so the caller must continue in a new transaction
// (decision E13′). Other transactions see no change: no NOWAIT and no
// lock_timeout.
func WithStatementTimeout(ctx context.Context, limit time.Duration, fn func(context.Context) error) error {
	if limit <= 0 {
		return fmt.Errorf("tenant: statement timeout must be positive, got %s", limit)
	}
	uow, err := unitOfWorkFromContext(ctx)
	if err != nil {
		return err
	}
	if uow.boundStatements == nil {
		return fmt.Errorf("%w: statement bound function is required", ErrRuntimeRequired)
	}
	return uow.boundStatements(ctx, limit, fn)
}
