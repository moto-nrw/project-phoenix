package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/protocol"
	"github.com/uptrace/bun"
)

type tenantTxKey struct{}
type adminTxKey struct{}
type traegerTxKey struct{}

type databaseTransactionKey struct{}

func transactionFromContext(ctx context.Context) (*bun.Tx, bool) {
	tx, ok := ctx.Value(databaseTransactionKey{}).(*bun.Tx)
	return tx, ok && tx != nil
}

func contextWithTransaction(ctx context.Context, tx *bun.Tx) context.Context {
	return context.WithValue(ctx, databaseTransactionKey{}, tx)
}

// ContextWithoutTransaction masks both adapter-owned transaction values.
func (r *PostgresUnitOfWork) ContextWithoutTransaction(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, databaseTransactionKey{}, nil)
	return ctx
}

// PostgresUnitOfWork is the PostgreSQL adapter for transaction execution.
// Tenant validation, retry ownership, and context propagation stay behind the
// tenant.UnitOfWork seam.
type PostgresUnitOfWork struct {
	db              *bun.DB
	observePoolWait func(context.Context, time.Duration)
	skipTenantRole  bool
}

func NewPostgresUnitOfWork(db *bun.DB, observePoolWait func(context.Context, time.Duration)) (*PostgresUnitOfWork, error) {
	if db == nil || observePoolWait == nil {
		return nil, fmt.Errorf("unit of work: database and pool-wait observer are required")
	}
	return &PostgresUnitOfWork{db: db, observePoolWait: observePoolWait}, nil
}

// NewDemoPostgresUnitOfWork preserves tenant RLS without granting the demo
// connection the broad phoenix_tenant role. The phoenix_demo login role has
// only the snapshot reads needed by the sidecar.
func NewDemoPostgresUnitOfWork(db *bun.DB, observePoolWait func(context.Context, time.Duration)) (*PostgresUnitOfWork, error) {
	unitOfWork, err := NewPostgresUnitOfWork(db, observePoolWait)
	if err != nil {
		return nil, err
	}
	unitOfWork.skipTenantRole = true
	return unitOfWork, nil
}

// rollbackDiagnostic is an operation error whose transaction ROLLBACK failed
// as well. The rollback failure is diagnostics in the text only: Unwrap
// yields the operation error, so retry, connectivity and error mapping decide
// on what the operation itself returned (#6316). The transaction ends either
// way: ROLLBACK or the broken connection discards it.
type rollbackDiagnostic struct {
	err      error
	rollback error
}

func (e *rollbackDiagnostic) Error() string {
	return e.err.Error() + "; unit of work: rollback: " + e.rollback.Error()
}

func (e *rollbackDiagnostic) Unwrap() error { return e.err }

// IsRetryableTransactionError classifies PostgreSQL deadlock and
// serialization failures. The UnitOfWork uses it only when the outer command
// explicitly opts into replay.
func IsRetryableTransactionError(err error) bool {
	state, ok := operationAs[interface{ Field(byte) string }](err)
	return ok && (state.Field('C') == "40P01" || state.Field('C') == "40001")
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
		// Both settings are transaction-local. No owner query runs until both
		// are set; the demo login retains its narrower role and grants.
		if r.skipTenantRole {
			if _, err := tx.NewRaw("SELECT set_config('app.current_tenant_id', ?, true)", fmt.Sprint(tenantID)).Exec(txCtx); err != nil {
				return fmt.Errorf("tenant runtime: set tenant ID: %w", err)
			}
		} else {
			if _, err := tx.NewRaw("SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_tenant_id', ?, true)", fmt.Sprint(tenantID)).Exec(txCtx); err != nil {
				return fmt.Errorf("tenant runtime: set tenant role: %w", err)
			}
		}

		txCtx = context.WithValue(txCtx, tenantTxKey{}, tenantID)
		txCtx = contextWithTransaction(txCtx, &tx)
		return fn(txCtx, tx)
	})
}

// WithinTraeger opens the Träger transaction of ADR 0086 section 5: the
// request role with app.current_org_id set and no school. The Träger policy
// of a Träger table admits only this Träger's rows; school tables, keyed by
// app.current_tenant_id, admit none. An ambient transaction is joined only
// when it is the Träger transaction of the same Träger.
func (r *PostgresUnitOfWork) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context, any) error) error {
	if organizationID <= 0 {
		return fmt.Errorf("tenant runtime: Träger ID must be positive")
	}
	if fn == nil {
		return fmt.Errorf("tenant runtime: callback is required")
	}
	if tx, ok := transactionFromContext(ctx); ok {
		active, isTraeger := ctx.Value(traegerTxKey{}).(int64)
		if !isTraeger || active != organizationID {
			return fmt.Errorf("tenant runtime: ambient transaction is not the Träger transaction")
		}
		return fn(ctx, *tx)
	}
	if r.skipTenantRole {
		return fmt.Errorf("tenant runtime: the demo runtime opens no Träger transaction")
	}
	return r.runInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(
			"SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_org_id', ?, true)",
			fmt.Sprint(organizationID),
		).Exec(txCtx); err != nil {
			return fmt.Errorf("tenant runtime: set Träger role: %w", err)
		}
		txCtx = context.WithValue(txCtx, traegerTxKey{}, organizationID)
		txCtx = contextWithTransaction(txCtx, &tx)
		return fn(txCtx, tx)
	})
}

// WithinAdminThenTenant implements the one-way handoff of ADR 0161 point 2.
// One transaction runs discover as phoenix_admin, then sets phoenix_tenant
// and app.current_tenant_id of the discovered school, both transaction-local
// like WithinTenant, and runs fn. An error from discover rolls back without
// the switch. fn's context carries only the tenant scope: an administrative
// join inside it fails as in any school unit, and even a retained
// administrative context runs as phoenix_tenant after the switch. An ambient
// transaction and the demo runtime are refused.
func (r *PostgresUnitOfWork) WithinAdminThenTenant(ctx context.Context, discover func(context.Context, any) (int64, error), fn func(context.Context, any) error) error {
	if discover == nil || fn == nil {
		return fmt.Errorf("tenant runtime: callbacks are required")
	}
	if _, ok := transactionFromContext(ctx); ok {
		return fmt.Errorf("tenant runtime: an admin-to-tenant handoff cannot join an ambient transaction")
	}
	if r.skipTenantRole {
		return fmt.Errorf("tenant runtime: the demo runtime opens no admin-to-tenant handoff")
	}
	return r.runInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(txCtx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return fmt.Errorf("tenant runtime: set admin role: %w", err)
		}
		tenantID, err := discover(contextWithTransaction(context.WithValue(txCtx, adminTxKey{}, true), &tx), tx)
		if err != nil {
			return err
		}
		if tenantID <= 0 {
			return fmt.Errorf("tenant runtime: tenant ID must be positive")
		}
		if _, err := tx.NewRaw("SELECT set_config('role', 'phoenix_tenant', true), set_config('app.current_tenant_id', ?, true)", fmt.Sprint(tenantID)).Exec(txCtx); err != nil {
			return fmt.Errorf("tenant runtime: set tenant role: %w", err)
		}
		return fn(contextWithTransaction(context.WithValue(txCtx, tenantTxKey{}, tenantID), &tx), tx)
	})
}

func (r *PostgresUnitOfWork) WithinAdmin(ctx context.Context, fn func(context.Context, any) error) error {
	return r.withinGlobal(ctx, true, fn)
}

// WithinDemo uses only the restricted demo login's existing grants for the
// cross-school queue/expiry capability. It must never assume phoenix_admin.
func (r *PostgresUnitOfWork) WithinDemo(ctx context.Context, fn func(context.Context, any) error) error {
	if !r.skipTenantRole {
		return fmt.Errorf("demo transaction requires the demo runtime")
	}
	return r.withinGlobal(ctx, false, fn)
}

func (r *PostgresUnitOfWork) withinGlobal(ctx context.Context, administrative bool, fn func(context.Context, any) error) error {
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
		if administrative {
			if _, err := tx.ExecContext(txCtx, "SET LOCAL ROLE phoenix_admin"); err != nil {
				return fmt.Errorf("tenant runtime: set admin role: %w", err)
			}
		}
		txCtx = context.WithValue(txCtx, adminTxKey{}, true)
		txCtx = contextWithTransaction(txCtx, &tx)
		return fn(txCtx, tx)
	})
}

func (r *PostgresUnitOfWork) runInTx(ctx context.Context, fn func(context.Context, bun.Tx) error) error {
	started := time.Now()
	conn, err := r.db.Conn(ctx)
	r.observePoolWait(ctx, time.Since(started))
	if err != nil {
		return &protocol.TransactionStartError{Err: err}
	}
	defer func() { _ = conn.Close() }()

	tx, err := conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return &protocol.TransactionStartError{Err: err}
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
			return &rollbackDiagnostic{err: err, rollback: rollbackErr}
		}
		return err
	}

	commitErr := tx.Commit()
	finished = true
	// database/sql may finish its automatic rollback before Commit observes
	// cancellation. Preserve the context error in either ordering; other commit
	// failures can still have an unknown durable outcome after cancellation.
	if errors.Is(commitErr, sql.ErrTxDone) && ctx.Err() != nil {
		commitErr = ctx.Err()
	}
	if commitErr == nil || IsRetryableTransactionError(commitErr) {
		return commitErr
	}
	return &protocol.CommitOutcomeUnknownError{Err: commitErr}
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

// BoundStatements runs fn with statement_timeout set to limit for the active
// transaction and restores the previous value after fn succeeds. Reading the
// previous value and setting the new one share one statement, restoring takes
// the second. When fn fails, the caller's enclosing savepoint rollback undoes
// the setting (tenant.WithStatementTimeout).
func (r *PostgresUnitOfWork) BoundStatements(ctx context.Context, limit time.Duration, fn func(context.Context) error) error {
	tx, err := savepointTx(ctx)
	if err != nil {
		return err
	}
	var previous, applied string
	if err := tx.NewRaw(`SELECT current_setting('statement_timeout'), set_config('statement_timeout', ?, true)`,
		fmt.Sprintf("%dms", limit.Milliseconds())).Scan(ctx, &previous, &applied); err != nil {
		return err
	}
	if err := fn(ctx); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `SELECT set_config('statement_timeout', ?, true)`, previous)
	return err
}

func savepointTx(ctx context.Context) (*bun.Tx, error) {
	tx, ok := transactionFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant runtime: transaction is required")
	}
	return tx, nil
}
