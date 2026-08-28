package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// provisionTenantRLS is the shared provisioning path for tenant isolation on
// newly created tenant-scoped tables. It applies exactly the contract migration
// 1.15.1 established for the original 58 tables: row level security enabled and
// forced, plus the canonical policy
//
//	tenant_isolation_<schema>_<table>
//
// matching tenant_id against the app.current_tenant_id session setting that
// TenantTxMiddleware sets per request.
//
// Every table passed in must be schema-qualified and carry a NOT NULL
// tenant_id.
//
// Why this lives in the migrations package: DDL is the only channel the project
// has for schema provisioning, and this DDL is not optional. The CLI connects
// as the postgres superuser, which bypasses RLS, but the HTTP server connects
// as phoenix_auth and switches to phoenix_tenant per request — for those roles
// the statements below ARE the tenant boundary. A tenant table that ships
// without them is readable across schools.
//
// Use this helper instead of hand-copying the four statements into each new
// migration; it keeps the policy shape and naming identical everywhere.
func provisionTenantRLS(ctx context.Context, db bun.IDB, tables ...string) error {
	for _, table := range tables {
		schema, name, ok := strings.Cut(table, ".")
		if !ok {
			return fmt.Errorf("provisionTenantRLS: table %q is not schema-qualified", table)
		}
		policy := fmt.Sprintf("tenant_isolation_%s_%s", schema, name)

		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s ENABLE ROW LEVEL SECURITY`, table)); err != nil {
			return fmt.Errorf("provisionTenantRLS: enable on %s: %w", table, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s FORCE ROW LEVEL SECURITY`, table)); err != nil {
			return fmt.Errorf("provisionTenantRLS: force on %s: %w", table, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			`DROP POLICY IF EXISTS %s ON %s`, policy, table)); err != nil {
			return fmt.Errorf("provisionTenantRLS: drop policy on %s: %w", table, err)
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			`CREATE POLICY %s ON %s FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)`,
			policy, table)); err != nil {
			return fmt.Errorf("provisionTenantRLS: create policy on %s: %w", table, err)
		}
	}

	return nil
}
