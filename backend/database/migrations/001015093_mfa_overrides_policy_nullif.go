package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mfaOverridesPolicyNullIfVersion     = "1.15.93"
	mfaOverridesPolicyNullIfDescription = "Wrap auth.mfa_overrides RLS policy in NULLIF so the policy survives unset app.current_tenant_id — fixes 22P02 during login-flow IsRequired (review-round-2 follow-up)"
)

// Existing deployments that ran 1.15.90 have an RLS policy that does
// `current_setting('app.current_tenant_id', true)::bigint` directly.
// The login flow (LoginWithMFAGate → IsRequired → FindGlobal) runs
// before any tenant context is established, so current_setting returns
// '' and ''::bigint raises 22P02 → IsRequired logs "global mfa override
// lookup failed; refusing login" → the API returns 503.
//
// This migration drops + recreates the policy with NULLIF so the right
// side resolves to NULL (never matches tenant_id) when the setting is
// unset, instead of crashing. The platform-wide row (tenant_id IS NULL)
// still resolves correctly.
//
// 1.15.90 has been patched in-source too, so a fresh `migrate reset`
// gets the same shape without this follow-up. This migration only
// matters for environments that already ran 1.15.90 in its original
// form.

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mfaOverridesPolicyNullIfVersion,
		Description: mfaOverridesPolicyNullIfDescription,
		DependsOn:   []string{mfaOverridesTenantScopeVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.93: rewriting auth.mfa_overrides RLS policy with NULLIF...")
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
				return fmt.Errorf("rewrite mfa_overrides RLS policy: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.93: restoring crash-on-unset policy shape...")
			_, err := db.ExecContext(ctx, `
				DROP POLICY IF EXISTS mfa_overrides_tenant_isolation ON auth.mfa_overrides;
				CREATE POLICY mfa_overrides_tenant_isolation ON auth.mfa_overrides
					USING (
						tenant_id IS NULL
						OR tenant_id = current_setting('app.current_tenant_id', true)::bigint
					)
					WITH CHECK (
						tenant_id IS NULL
						OR tenant_id = current_setting('app.current_tenant_id', true)::bigint
					);
			`)
			if err != nil {
				return fmt.Errorf("rollback mfa_overrides RLS policy: %w", err)
			}
			return nil
		},
	)
}
