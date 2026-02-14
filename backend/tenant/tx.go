package tenant

import (
	"context"
	"database/sql"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// WithTenantTx executes fn inside a transaction that:
//  1. Sets the session role to phoenix_tenant (RLS-enforced)
//  2. Sets app.current_tenant_id via set_config (visible to RLS policies)
//
// The role and config are LOCAL to the transaction and automatically reset on commit/rollback.
func WithTenantTx(ctx context.Context, db *bun.DB, tenantID int64, fn func(ctx context.Context, tx bun.Tx) error) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant: WithTenantTx requires a non-zero tenant_id")
	}

	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Switch to the RLS-enforced tenant role
		if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant"); err != nil {
			return fmt.Errorf("tenant: SET LOCAL ROLE phoenix_tenant: %w", err)
		}

		// Set the tenant ID so RLS policies can read it via current_setting('app.current_tenant_id')
		if _, err := tx.ExecContext(ctx,
			"SELECT set_config('app.current_tenant_id', $1, true)",
			fmt.Sprintf("%d", tenantID),
		); err != nil {
			return fmt.Errorf("tenant: set_config app.current_tenant_id: %w", err)
		}

		// Store tx in context so GetDB(ctx, r.db) finds it (CRIT-1 bridge)
		ctx = modelBase.ContextWithTx(ctx, &tx)

		return fn(ctx, tx)
	})
}

// WithAdminTx executes fn inside a transaction that:
//  1. Sets the session role to phoenix_admin (BYPASSRLS)
//
// Use this for cross-tenant operations like migrations, platform-level queries, or data exports.
func WithAdminTx(ctx context.Context, db *bun.DB, fn func(ctx context.Context, tx bun.Tx) error) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Switch to the admin role that bypasses RLS
		if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return fmt.Errorf("tenant: SET LOCAL ROLE phoenix_admin: %w", err)
		}

		// Store tx in context so GetDB(ctx, r.db) finds it (CRIT-1 bridge)
		ctx = modelBase.ContextWithTx(ctx, &tx)

		return fn(ctx, tx)
	})
}
