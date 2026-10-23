package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const authTenantRLSVersion = "1.15.475"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     authTenantRLSVersion,
		Description: "Enforce school isolation on auth tenant tables found by the RLS coverage guard (#3842)",
		DependsOn:   []string{"1.15.449", "1.15.282", "1.15.133", "1.15.91", "1.14.6"},
	})
	Migrations.MustRegister(authTenantRLSUp, authTenantRLSDown)
}

func authTenantRLSUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := provisionTenantRLS(ctx, tx,
			"auth.account_tenants", "auth.mfa_email_challenges", "auth.mfa_trusted_devices",
			"auth.passkey_sessions", "auth.parent_declaration_permission_grants",
		); err != nil {
			return err
		}
		// Login, MFA and passkey ceremonies run before a school transaction
		// exists. Preserve the base role's existing grants, not tenant access:
		// phoenix_tenant does not inherit phoenix_auth. Operator/worker access
		// remains on phoenix_admin (BYPASSRLS). No row is rewritten or removed.
		if _, err := tx.ExecContext(ctx, `
			CREATE POLICY pre_auth_account_tenants ON auth.account_tenants
				FOR ALL TO phoenix_auth USING (true) WITH CHECK (true);
			CREATE POLICY pre_auth_mfa_email_challenges ON auth.mfa_email_challenges
				FOR ALL TO phoenix_auth USING (true) WITH CHECK (true);
			CREATE POLICY pre_auth_mfa_trusted_devices ON auth.mfa_trusted_devices
				FOR ALL TO phoenix_auth USING (true) WITH CHECK (true);
			CREATE POLICY pre_auth_passkey_sessions ON auth.passkey_sessions
				FOR ALL TO phoenix_auth USING (true) WITH CHECK (true);

			-- Offboarding must not deactivate a shared account at other schools.
			-- Expose only active school IDs of an account active at the caller's
			-- own school, never another school's membership row or feed token.
			CREATE FUNCTION auth.active_school_ids_for_account(p_account_id BIGINT)
			RETURNS TABLE(tenant_id BIGINT)
			LANGUAGE SQL STABLE SECURITY DEFINER SET search_path = pg_catalog, auth
			AS $function$
				SELECT mapping.tenant_id FROM auth.account_tenants AS mapping
				WHERE mapping.account_id = p_account_id AND mapping.status = 'active'
				AND (
					current_setting('role') IN ('phoenix_auth', 'phoenix_admin')
					OR (current_setting('role') = 'none' AND (
						pg_has_role(session_user, 'phoenix_auth', 'USAGE')
						OR pg_has_role(session_user, 'phoenix_admin', 'USAGE')))
					OR EXISTS (
						SELECT FROM auth.account_tenants AS own
						WHERE own.account_id = p_account_id AND own.status = 'active'
						AND own.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					)
				)
				ORDER BY mapping.created_at, mapping.tenant_id
			$function$;
			REVOKE ALL ON FUNCTION auth.active_school_ids_for_account(BIGINT) FROM PUBLIC;
			GRANT EXECUTE ON FUNCTION auth.active_school_ids_for_account(BIGINT)
				TO phoenix_tenant, phoenix_auth, phoenix_admin;
		`); err != nil {
			return fmt.Errorf("preserve pre-auth access: %w", err)
		}
		return nil
	})
}

func authTenantRLSDown(_ context.Context, _ *bun.DB) error {
	return fmt.Errorf("refusing to remove auth tenant RLS: rollback would restore cross-school access; use a forward fix, and verify RLS compatibility before deploying an older application")
}
