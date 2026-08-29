package database

import (
	"context"
	"database/sql"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type tenantTxKey struct{}
type adminTxKey struct{}

const (
	createSavepoint uint8 = iota + 1
	rollbackSavepoint
	releaseSavepoint
)

// TenantRuntime implements the PostgreSQL half of tenant runtime execution.
// Tenant validation and context propagation stay in package tenant.
type TenantRuntime struct {
	db *bun.DB
}

func NewTenantRuntime(db *bun.DB) (*TenantRuntime, error) {
	if db == nil {
		return nil, fmt.Errorf("tenant runtime: database is required")
	}
	return &TenantRuntime{db: db}, nil
}

func (r *TenantRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context, any) error) error {
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

	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(txCtx context.Context, tx bun.Tx) error {
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

func (r *TenantRuntime) WithinAdmin(ctx context.Context, fn func(context.Context, any) error) error {
	if fn == nil {
		return fmt.Errorf("tenant runtime: callback is required")
	}
	if tx, ok := modelBase.TxFromContext(ctx); ok {
		if active, _ := ctx.Value(adminTxKey{}).(bool); !active {
			return fmt.Errorf("tenant runtime: ambient transaction is not administrative")
		}
		return fn(modelBase.ContextWithTx(ctx, tx), *tx)
	}
	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(txCtx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(txCtx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return fmt.Errorf("tenant runtime: set admin role: %w", err)
		}
		txCtx = context.WithValue(txCtx, adminTxKey{}, true)
		return fn(modelBase.ContextWithTx(txCtx, &tx), tx)
	})
}

func (r *TenantRuntime) ControlSavepoint(ctx context.Context, action uint8) error {
	tx, ok := modelBase.TxFromContext(ctx)
	if !ok {
		return fmt.Errorf("tenant runtime: transaction is required")
	}
	var err error
	switch action {
	case createSavepoint:
		_, err = tx.ExecContext(ctx, "SAVEPOINT phoenix_operation")
	case rollbackSavepoint:
		_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT phoenix_operation")
	case releaseSavepoint:
		_, err = tx.ExecContext(ctx, "RELEASE SAVEPOINT phoenix_operation")
	default:
		return fmt.Errorf("tenant runtime: unknown savepoint action %d", action)
	}
	return err
}
