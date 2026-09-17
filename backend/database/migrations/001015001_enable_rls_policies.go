package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	enableRLSVersion     = "1.15.1"
	enableRLSDescription = "Enable Row Level Security and create tenant isolation policies on all tenant-scoped tables"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enableRLSVersion,
		Description: enableRLSDescription,
		DependsOn:   []string{"1.14.5", "1.14.6"}, // indexes + account_tenants populated
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return enableRLSPolicies(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackRLSPolicies(ctx, db)
		},
	)
}

func enableRLSPolicies(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// Enable RLS on all 58 NOT NULL tenant_id tables
	for _, table := range tablesWithNotNullTenantID {
		parts := strings.SplitN(table, ".", 2)
		schema, tableName := parts[0], parts[1]
		policyName := fmt.Sprintf("tenant_isolation_%s_%s", schema, tableName)

		// Enable and force RLS
		_, err := tx.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s ENABLE ROW LEVEL SECURITY`, table))
		if err != nil {
			return fmt.Errorf("failed to enable RLS on %s: %w", table, err)
		}

		_, err = tx.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s FORCE ROW LEVEL SECURITY`, table))
		if err != nil {
			return fmt.Errorf("failed to force RLS on %s: %w", table, err)
		}

		// Create tenant isolation policy
		_, err = tx.ExecContext(ctx, fmt.Sprintf(
			`CREATE POLICY %s ON %s FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)`,
			policyName, table))
		if err != nil {
			return fmt.Errorf("failed to create policy on %s: %w", table, err)
		}
	}

	// Special case: auth.roles (nullable tenant_id)
	// System roles (tenant_id IS NULL) are visible to all tenants.
	// Tenant-scoped roles (tenant_id IS NOT NULL) are only visible to that tenant.
	_, err = tx.ExecContext(ctx, `ALTER TABLE auth.roles ENABLE ROW LEVEL SECURITY`)
	if err != nil {
		return fmt.Errorf("failed to enable RLS on auth.roles: %w", err)
	}
	_, err = tx.ExecContext(ctx, `ALTER TABLE auth.roles FORCE ROW LEVEL SECURITY`)
	if err != nil {
		return fmt.Errorf("failed to force RLS on auth.roles: %w", err)
	}
	_, err = tx.ExecContext(ctx, `CREATE POLICY tenant_isolation_auth_roles ON auth.roles FOR ALL
		USING (
			tenant_id IS NULL
			OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		)
		WITH CHECK (
			tenant_id IS NULL
			OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
		)`)
	if err != nil {
		return fmt.Errorf("failed to create policy on auth.roles: %w", err)
	}

	migrationLog().InfoContext(ctx, "row level security enabled with tenant isolation policies",
		"tables", len(tablesWithNotNullTenantID)+1,
	)
	return tx.Commit()
}

func rollbackRLSPolicies(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// Remove policies and disable RLS on all 58 NOT NULL tables
	for _, table := range tablesWithNotNullTenantID {
		parts := strings.SplitN(table, ".", 2)
		schema, tableName := parts[0], parts[1]
		policyName := fmt.Sprintf("tenant_isolation_%s_%s", schema, tableName)

		_, _ = tx.ExecContext(ctx, fmt.Sprintf(`DROP POLICY IF EXISTS %s ON %s`, policyName, table))
		_, _ = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DISABLE ROW LEVEL SECURITY`, table))
	}

	// auth.roles
	_, _ = tx.ExecContext(ctx, `DROP POLICY IF EXISTS tenant_isolation_auth_roles ON auth.roles`)
	_, _ = tx.ExecContext(ctx, `ALTER TABLE auth.roles DISABLE ROW LEVEL SECURITY`)

	return tx.Commit()
}
