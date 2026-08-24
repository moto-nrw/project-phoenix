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
// Nested-safe: if the context already contains a tenant tx with the same tenantID, the existing
// tx is reused instead of opening a new one. Mismatched tenantIDs return an error.
//
// After-commit hooks: callbacks registered via RegisterAfterCommit run only
// after the OUTERMOST WithTenantTx successfully commits. Nested calls just
// append to the shared holder. If db.RunInTx returns an error (commit failed
// or fn errored), queued hooks are dropped — destructive side effects MUST
// NOT run when the database state never moved.
func WithTenantTx(ctx context.Context, db *bun.DB, tenantID int64, fn func(ctx context.Context, tx bun.Tx) error) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant: WithTenantTx requires a non-zero tenant_id")
	}

	// Nested-safe: if already in a tenant tx, reuse it. The hooks holder is
	// already attached to ctx by the outermost call — drains happen there.
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		existingTenantID := FromContext(ctx)
		if existingTenantID != tenantID {
			return fmt.Errorf("tenant: nested WithTenantTx with mismatched tenant_id (%d vs %d)", existingTenantID, tenantID)
		}
		return fn(ctx, *tx)
	}

	// Outermost call: attach a fresh hooks holder so nested
	// RegisterAfterCommit appends to it via context propagation.
	ctx, commitHooks := withAfterCommitHooks(ctx)

	err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Switch to the RLS-enforced tenant role
		if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant"); err != nil {
			return fmt.Errorf("tenant: SET LOCAL ROLE phoenix_tenant: %w", err)
		}

		// Set the tenant ID so RLS policies can read it via current_setting().
		// Use fmt.Sprintf instead of a $1 parameter because bun.Tx.ExecContext
		// doesn't forward positional parameters to the driver correctly. Safe:
		// tenantID is int64, never user input.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(
			"SELECT set_config('app.current_tenant_id', '%d', true)",
			tenantID,
		)); err != nil {
			return fmt.Errorf("tenant: set_config tenant: %w", err)
		}

		// Store tenant ID in Go context so tenant.FromContext(ctx) works
		// inside services called from scheduler or other non-HTTP contexts
		ctx = WithTenantID(ctx, tenantID)

		// Store tx in context so GetDB(ctx, r.db) finds it (CRIT-1 bridge)
		ctx = modelBase.ContextWithTx(ctx, &tx)

		return fn(ctx, tx)
	})
	if err != nil {
		return err
	}
	runAfterCommitHooks(commitHooks)
	return nil
}

// WithAdminTx executes fn inside a transaction that:
//  1. Sets the session role to phoenix_admin (BYPASSRLS)
//
// Use this for cross-tenant operations like migrations, platform-level queries, or data exports.
func WithAdminTx(ctx context.Context, db *bun.DB, fn func(ctx context.Context, tx bun.Tx) error) error {
	ctx = ContextWithoutAfterCommitHooks(ctx)
	ctx, commitHooks := withAfterCommitHooks(ctx)
	err := db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		// Switch to the admin role that bypasses RLS
		if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_admin"); err != nil {
			return fmt.Errorf("tenant: SET LOCAL ROLE phoenix_admin: %w", err)
		}

		// Store tx in context so GetDB(ctx, r.db) finds it (CRIT-1 bridge)
		ctx = modelBase.ContextWithTx(ctx, &tx)
		ctx = withAdminTxFlag(ctx)

		return fn(ctx, tx)
	})
	if err != nil {
		return err
	}
	runAfterCommitHooks(commitHooks)
	return nil
}

// WithAdminTxOrDirect runs fn inside a phoenix_admin (BYPASSRLS) transaction
// like WithAdminTx, degrading gracefully for test wiring: when the context
// already carries a transaction, or db is nil/unusable (zero-value &bun.DB{}
// stubs panic on any operation), fn runs directly instead. Replaces the
// per-service private withAdminTx shims (audit B7).
func WithAdminTxOrDirect(ctx context.Context, db *bun.DB, fn func(ctx context.Context) error) error {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return fn(ctx)
	}
	if !usableDB(db) {
		return fn(ctx)
	}
	return WithAdminTx(ctx, db, func(adminCtx context.Context, _ bun.Tx) error {
		return fn(adminCtx)
	})
}

// usableDB reports whether db is a real connection. A zero-value &bun.DB{}
// (common in unit-test struct literals) panics on any operation; probe it
// with a harmless stats call.
func usableDB(db *bun.DB) (ok bool) {
	if db == nil {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	_ = db.DBStats()
	return true
}
