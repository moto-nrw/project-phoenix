package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mfaOverridesRestrictGlobalWriteVersion     = "1.15.92"
	mfaOverridesRestrictGlobalWriteDescription = "Restrict auth.mfa_overrides RLS WITH CHECK so only phoenix_admin (BYPASSRLS) can write platform-wide rows — RLS roles can no longer self-insert a tenant_id IS NULL override (#1430 review round 3, finding ②)"
)

// The 1.15.90 policy allowed `tenant_id IS NULL` in WITH CHECK, which let
// ANY RLS role (incl. phoenix_tenant) write a platform-wide override row
// directly. The service layer gates this (OperatorSetGlobalMFAOverride is
// operator-only), but the DB policy was missing as defense-in-depth: a
// compromised or misused tenant-scoped DB connection could set a global
// force_off on any account.
//
// Fix: drop `tenant_id IS NULL` from WITH CHECK. Now an RLS role may only
// write a row whose tenant_id equals its own app.current_tenant_id.
// Platform-wide rows (tenant_id IS NULL) can only be written by
// phoenix_admin, which bypasses RLS entirely — exactly the role
// OperatorSetGlobalMFAOverride runs under via tenant.WithAdminTx.
//
// USING (read visibility) is unchanged: every tenant must still SEE the
// platform-wide row so IsRequired's FindGlobal resolves it.

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mfaOverridesRestrictGlobalWriteVersion,
		Description: mfaOverridesRestrictGlobalWriteDescription,
		DependsOn:   []string{mfaOverridesPolicyNullIfVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.92: restricting auth.mfa_overrides WITH CHECK to tenant-scoped writes...")
			_, err := db.ExecContext(ctx, `
				DROP POLICY IF EXISTS mfa_overrides_tenant_isolation ON auth.mfa_overrides;
				CREATE POLICY mfa_overrides_tenant_isolation ON auth.mfa_overrides
					USING (
						-- Read: platform-wide rows visible to every tenant so
						-- IsRequired's FindGlobal resolves them. (NULLIF guards
						-- the unset-tenant login path — see 1.15.91.)
						tenant_id IS NULL
						OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					)
					WITH CHECK (
						-- Write: an RLS role may only write its OWN tenant's row.
						-- Platform-wide (tenant_id IS NULL) writes require
						-- phoenix_admin (BYPASSRLS) — the operator-only path.
						tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					);
			`)
			if err != nil {
				return fmt.Errorf("restrict mfa_overrides WITH CHECK: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.92: restoring permissive WITH CHECK (tenant_id IS NULL allowed)...")
			_, err := db.ExecContext(ctx, `
				DROP POLICY IF EXISTS mfa_overrides_tenant_isolation ON auth.mfa_overrides;
				CREATE POLICY mfa_overrides_tenant_isolation ON auth.mfa_overrides
					USING (
						tenant_id IS NULL
						OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					)
					WITH CHECK (
						tenant_id IS NULL
						OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					);
			`)
			if err != nil {
				return fmt.Errorf("rollback mfa_overrides WITH CHECK: %w", err)
			}
			return nil
		},
	)
}
