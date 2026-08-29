package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type tenantTxKey struct{}
type adminTxKey struct{}

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
	return modelBase.IsRetryableTxError(err)
}

func (r *PostgresUnitOfWork) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context, any) error) error {
	if tenantID <= 0 {
		return fmt.Errorf("tenant runtime: tenant ID must be positive")
	}
	if fn == nil {
		return fmt.Errorf("tenant runtime: callback is required")
	}

	if tx, ok := modelBase.TxFromContext(ctx); ok {
		activeTenantID, active := ctx.Value(tenantTxKey{}).(int64)
		if !active || activeTenantID != tenantID {
			return fmt.Errorf("tenant runtime: ambient transaction has no matching tenant")
		}
		return fn(modelBase.ContextWithTx(ctx, tx), *tx)
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
		txCtx = modelBase.ContextWithTx(txCtx, &tx)
		return fn(txCtx, tx)
	})
}

func (r *PostgresUnitOfWork) WithinAdmin(ctx context.Context, fn func(context.Context, any) error) error {
	if fn == nil {
		return fmt.Errorf("tenant runtime: callback is required")
	}
	if tx, ok := modelBase.TxFromContext(ctx); ok {
		if active, _ := ctx.Value(adminTxKey{}).(bool); !active {
			return fmt.Errorf("tenant runtime: ambient transaction is not administrative")
		}
		return fn(modelBase.ContextWithTx(ctx, tx), *tx)
	}
	return r.runInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(txCtx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return fmt.Errorf("tenant runtime: set admin role: %w", err)
		}
		txCtx = context.WithValue(txCtx, adminTxKey{}, true)
		return fn(modelBase.ContextWithTx(txCtx, &tx), tx)
	})
}

func (r *PostgresUnitOfWork) runInTx(ctx context.Context, fn func(context.Context, bun.Tx) error) error {
	started := time.Now()
	conn, err := r.db.Conn(ctx)
	r.observePoolWait(ctx, time.Since(started))
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	return conn.RunInTx(ctx, &sql.TxOptions{}, fn)
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

func savepointTx(ctx context.Context) (*bun.Tx, error) {
	tx, ok := modelBase.TxFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("tenant runtime: transaction is required")
	}
	return tx, nil
}
