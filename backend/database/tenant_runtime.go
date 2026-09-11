package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type tenantTxKey struct{}
type adminTxKey struct{}

type databaseTransactionKey struct{}

func transactionFromContext(ctx context.Context) (*bun.Tx, bool) {
	tx, ok := ctx.Value(databaseTransactionKey{}).(*bun.Tx)
	return tx, ok && tx != nil
}

func contextWithTransaction(ctx context.Context, tx *bun.Tx) context.Context {
	return context.WithValue(ctx, databaseTransactionKey{}, tx)
}

func (r *PostgresUnitOfWork) ContextWithTenant(ctx context.Context, tenantID int64) context.Context {
	return modelBase.WithRepositoryTenantID(ctx, tenantID)
}

func (r *PostgresUnitOfWork) ContextWithTransaction(ctx context.Context, transaction any) context.Context {
	switch tx := transaction.(type) {
	case nil:
		return modelBase.WithoutRepositoryTransaction(ctx)
	case bun.Tx:
		return modelBase.WithRepositoryContext(ctx, modelBase.RepositoryTenantID(ctx), tx)
	case *bun.Tx:
		if tx != nil {
			return modelBase.WithRepositoryContext(ctx, modelBase.RepositoryTenantID(ctx), tx)
		}
		return modelBase.WithoutRepositoryTransaction(ctx)
	default:
		panic(fmt.Sprintf("unit of work: unsupported repository transaction type %T", transaction))
	}
}

// ContextWithoutTransaction masks both adapter-owned transaction values.
func (r *PostgresUnitOfWork) ContextWithoutTransaction(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, databaseTransactionKey{}, nil)
	return modelBase.WithoutRepositoryTransaction(ctx)
}

type transactionStartError struct{ err error }

func (e *transactionStartError) Error() string {
	return "unit of work: start transaction: " + e.err.Error()
}
func (e *transactionStartError) Unwrap() error          { return e.err }
func (e *transactionStartError) TransactionNotStarted() {}

type commitOutcomeUnknownError struct{ err error }

func (e *commitOutcomeUnknownError) Error() string {
	return "unit of work: commit outcome unknown: " + e.err.Error()
}
func (e *commitOutcomeUnknownError) Unwrap() error         { return e.err }
func (e *commitOutcomeUnknownError) CommitOutcomeUnknown() {}

// PostgresUnitOfWork is the PostgreSQL adapter for transaction execution.
// Tenant validation, retry ownership, and context propagation stay behind the
// tenant.UnitOfWork seam.
type PostgresUnitOfWork struct {
	db              *bun.DB
	observePoolWait func(context.Context, time.Duration)
}

func NewPostgresUnitOfWork(db *bun.DB, observePoolWait func(context.Context, time.Duration)) (*PostgresUnitOfWork, error) {
	if db == nil || observePoolWait == nil {
		return nil, fmt.Errorf("unit of work: database and pool-wait observer are required")
	}
	return &PostgresUnitOfWork{db: db, observePoolWait: observePoolWait}, nil
}

// IsRetryableTransactionError classifies PostgreSQL deadlock and
// serialization failures. The UnitOfWork uses it only when the outer command
// explicitly opts into replay.
func IsRetryableTransactionError(err error) bool {
	var state interface{ Field(byte) string }
	return errors.As(err, &state) && (state.Field('C') == "40P01" || state.Field('C') == "40001")
}

func (r *PostgresUnitOfWork) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context, any) error) error {
	if tenantID <= 0 {
		return fmt.Errorf("tenant runtime: tenant ID must be positive")
	}
	if fn == nil {
		return fmt.Errorf("tenant runtime: callback is required")
	}

	if tx, ok := transactionFromContext(ctx); ok {
		activeTenantID, active := ctx.Value(tenantTxKey{}).(int64)
		if !active || activeTenantID != tenantID {
			return fmt.Errorf("tenant runtime: ambient transaction has no matching tenant")
		}
		return fn(ctx, *tx)
	}

	return r.runInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(txCtx, "SET LOCAL ROLE phoenix_tenant"); err != nil {
			return fmt.Errorf("tenant runtime: set tenant role: %w", err)
		}
		if _, err := tx.NewRaw(
			"SELECT set_config('app.current_tenant_id', ?, true)",
			fmt.Sprint(tenantID),
		).Exec(txCtx); err != nil {
			return fmt.Errorf("tenant runtime: set tenant ID: %w", err)
		}

		txCtx = context.WithValue(txCtx, tenantTxKey{}, tenantID)
		txCtx = contextWithTransaction(txCtx, &tx)
		txCtx = modelBase.WithRepositoryContext(txCtx, tenantID, tx)
		return fn(txCtx, tx)
	})
}

func (r *PostgresUnitOfWork) WithinAdmin(ctx context.Context, fn func(context.Context, any) error) error {
	if fn == nil {
		return fmt.Errorf("tenant runtime: callback is required")
	}
	if tx, ok := transactionFromContext(ctx); ok {
		if active, _ := ctx.Value(adminTxKey{}).(bool); !active {
			return fmt.Errorf("tenant runtime: ambient transaction is not administrative")
		}
		return fn(ctx, *tx)
	}
	return r.runInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(txCtx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return fmt.Errorf("tenant runtime: set admin role: %w", err)
		}
		txCtx = context.WithValue(txCtx, adminTxKey{}, true)
		txCtx = contextWithTransaction(txCtx, &tx)
		txCtx = modelBase.WithRepositoryContext(txCtx, 0, tx)
		return fn(txCtx, tx)
	})
}

func (r *PostgresUnitOfWork) runInTx(ctx context.Context, fn func(context.Context, bun.Tx) error) error {
	started := time.Now()
	conn, err := r.db.Conn(ctx)
	r.observePoolWait(ctx, time.Since(started))
	if err != nil {
		return &transactionStartError{err: err}
	}
	defer func() { _ = conn.Close() }()

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return &transactionStartError{err: err}
	}
	finished := false
	defer func() {
		if !finished {
			_ = tx.Rollback()
		}
	}()

	if err := fn(ctx, tx); err != nil {
		rollbackErr := tx.Rollback()
		finished = true
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("unit of work: rollback: %w", rollbackErr))
		}
		return err
	}

	commitErr := tx.Commit()
	finished = true
	if commitErr == nil || IsRetryableTransactionError(commitErr) {
		return commitErr
	}
	return &commitOutcomeUnknownError{err: commitErr}
}

// The three savepoint methods implement tenant.SavepointController.

func (r *PostgresUnitOfWork) CreateSavepoint(ctx context.Context) error {
	tx, err := savepointTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "SAVEPOINT phoenix_operation")
	return err
}

func (r *PostgresUnitOfWork) RollbackSavepoint(ctx context.Context) error {
	tx, err := savepointTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT phoenix_operation")
	return err
}

func (r *PostgresUnitOfWork) ReleaseSavepoint(ctx context.Context) error {
	tx, err := savepointTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "RELEASE SAVEPOINT phoenix_operation")
	return err
}

// AcquireLock takes a transaction-scoped advisory lock through the active
// PostgreSQL transaction.
func (r *PostgresUnitOfWork) AcquireLock(ctx context.Context, key string, shared bool) error {
	tx, ok := transactionFromContext(ctx)
	if !ok {
		return fmt.Errorf("tenant runtime: transaction is required")
	}
	if shared {
		_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock_shared(hashtextextended(?, 0))", key)
		return err
	}
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	return err
}

func savepointTx(ctx context.Context) (*bun.Tx, error) {
	tx, ok := transactionFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant runtime: transaction is required")
	}
	return tx, nil
}
