package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mfaOverridesTenantScopeVersion     = "1.15.90"
	mfaOverridesTenantScopeDescription = "Replace auth.accounts.mfa_admin_override (global) with auth.mfa_overrides (tenant-scoped + optional platform-wide row from operator) — closes #1430 review-round-2 Blocker on cross-tenant override bleed."
)

// The store shape:
//
//   auth.mfa_overrides (account_id, tenant_id NULL = platform-wide, override, ...)
//
// Resolution order in IsRequired:
//   1. Row with tenant_id IS NULL  → platform-wide override set by operator
//   2. Row with tenant_id = login tenant → tenant-admin override
//   3. → no override; tenant policy decides
//
// Why nullable tenant_id over two tables:
//   - One repo, one resolver, one audit shape.
//   - Postgres partial unique indexes give us precise constraints
//     (one platform-wide row per account, one tenant-scoped row per
//     (account, tenant)) without sentinel values.
//
// Why partial unique indexes over `NULLS NOT DISTINCT`:
//   - `NULLS NOT DISTINCT` would let us write the constraint in one
//     line on Postgres 15+, but partial indexes make the *intent*
//     explicit at the schema level: a global row is a different kind
//     of row from a tenant row, not just a NULL-valued one.

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mfaOverridesTenantScopeVersion,
		Description: mfaOverridesTenantScopeDescription,
		DependsOn:   []string{mfaTrustedDevicesTenantScopeVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.90: creating auth.mfa_overrides + migrating auth.accounts.mfa_admin_override...")
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS auth.mfa_overrides (
					id          BIGSERIAL PRIMARY KEY,
					account_id  BIGINT NOT NULL,
					-- NULL == platform-wide override set by the operator. The
					-- operator surface uses this for the "mailbox lockout"
					-- emergency switch that bypasses MFA across every tenant
					-- the account belongs to. Tenant admins can never write
					-- a NULL row.
					tenant_id   BIGINT,
					override    TEXT NOT NULL,
					-- Who set it. set_by_type distinguishes tenant-admin
					-- (account_id from auth.accounts) from operator
					-- (operator_id from platform.operators). The two ID
					-- spaces overlap so we record the type alongside the ID.
					set_by      BIGINT NOT NULL,
					set_by_type TEXT NOT NULL,
					reason      TEXT NOT NULL,
					created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					CONSTRAINT fk_mfa_overrides_account
						FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE,
					CONSTRAINT fk_mfa_overrides_tenant
						FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
					CONSTRAINT chk_mfa_overrides_value
						CHECK (override IN ('force_off', 'force_on')),
					CONSTRAINT chk_mfa_overrides_set_by_type
						CHECK (set_by_type IN ('account', 'operator'))
				);

				-- One platform-wide row per account.
				CREATE UNIQUE INDEX IF NOT EXISTS uq_mfa_overrides_global
					ON auth.mfa_overrides (account_id)
					WHERE tenant_id IS NULL;

				-- One tenant-scoped row per (account, tenant).
				CREATE UNIQUE INDEX IF NOT EXISTS uq_mfa_overrides_tenant
					ON auth.mfa_overrides (account_id, tenant_id)
					WHERE tenant_id IS NOT NULL;

				-- Lookup index for the IsRequired resolver. The (account_id,
				-- tenant_id) order covers both the platform-wide and
				-- tenant-specific probes — Postgres can use it for
				-- "WHERE account_id = ? AND tenant_id IS NULL" and for
				-- "WHERE account_id = ? AND tenant_id = ?" alike.
				CREATE INDEX IF NOT EXISTS idx_mfa_overrides_lookup
					ON auth.mfa_overrides (account_id, tenant_id);

				DROP TRIGGER IF EXISTS update_mfa_overrides_updated_at ON auth.mfa_overrides;
				CREATE TRIGGER update_mfa_overrides_updated_at
				BEFORE UPDATE ON auth.mfa_overrides
				FOR EACH ROW
				EXECUTE FUNCTION update_modified_column();

				-- Data migration: any existing non-default value on the
				-- old global column becomes a platform-wide row. There is
				-- no way to determine which tenant the original setter
				-- intended, and the safer interpretation is "operator
				-- intent" (cross-tenant) rather than a guessed tenant.
				-- Production never ran 1.15.88 (the column is feature-
				-- branch only), so in practice this only catches dev DBs.
				-- set_by = 0 is the sentinel for migrated rows since the
				-- original column did not record the setter.
				INSERT INTO auth.mfa_overrides
					(account_id, tenant_id, override, set_by, set_by_type, reason)
				SELECT id, NULL, mfa_admin_override, 0, 'operator',
				       'migrated from auth.accounts.mfa_admin_override'
				FROM auth.accounts
				WHERE mfa_admin_override IN ('force_off', 'force_on');

				ALTER TABLE auth.accounts DROP CONSTRAINT IF EXISTS chk_accounts_mfa_admin_override;
				ALTER TABLE auth.accounts DROP COLUMN IF EXISTS mfa_admin_override;

				-- RLS lets us treat the platform-wide rows as visible to all
				-- tenant connections (read-only) while keeping tenant-scoped
				-- rows isolated to their owner tenant. Writes are gated at
				-- the service layer; the policy here is defence in depth.
				ALTER TABLE auth.mfa_overrides ENABLE ROW LEVEL SECURITY;
				ALTER TABLE auth.mfa_overrides FORCE ROW LEVEL SECURITY;

				DROP POLICY IF EXISTS mfa_overrides_tenant_isolation ON auth.mfa_overrides;
				CREATE POLICY mfa_overrides_tenant_isolation ON auth.mfa_overrides
					USING (
						-- Platform-wide rows are visible to every tenant
						-- (the operator sets them and they affect every
						-- login regardless of tenant).
						--
						-- NULLIF guards against the login flow where
						-- app.current_tenant_id is unset — current_setting
						-- returns '' there, and ''::bigint raises 22P02. The
						-- NULLIF makes the right side resolve to NULL (which
						-- never matches tenant_id), so the policy still
						-- returns the platform-wide rows but doesn't crash
						-- on the unset case. (#1430 review round 2 follow-
						-- up: IsRequired needs FindGlobal to be callable
						-- before any tenant context is established.)
						tenant_id IS NULL
						OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					)
					WITH CHECK (
						tenant_id IS NULL
						OR tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
					);

				GRANT SELECT, INSERT, UPDATE, DELETE
					ON auth.mfa_overrides TO PUBLIC;
				GRANT USAGE ON SEQUENCE auth.mfa_overrides_id_seq TO PUBLIC;
			`)
			if err != nil {
				return fmt.Errorf("create auth.mfa_overrides + drop auth.accounts.mfa_admin_override: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.90: restoring auth.accounts.mfa_admin_override + dropping auth.mfa_overrides...")
			_, err := db.ExecContext(ctx, `
				ALTER TABLE auth.accounts
				ADD COLUMN IF NOT EXISTS mfa_admin_override TEXT NOT NULL DEFAULT 'none';

				ALTER TABLE auth.accounts
				ADD CONSTRAINT chk_accounts_mfa_admin_override
				CHECK (mfa_admin_override IN ('none', 'force_off', 'force_on'));

				-- Reverse migration: copy platform-wide rows back onto
				-- auth.accounts. Tenant-scoped rows are dropped on rollback
				-- (the old shape couldn't represent them).
				UPDATE auth.accounts a
				SET mfa_admin_override = o.override
				FROM auth.mfa_overrides o
				WHERE o.account_id = a.id AND o.tenant_id IS NULL;

				DROP TRIGGER IF EXISTS update_mfa_overrides_updated_at ON auth.mfa_overrides;
				DROP TABLE IF EXISTS auth.mfa_overrides CASCADE;
			`)
			if err != nil {
				return fmt.Errorf("rollback auth.mfa_overrides: %w", err)
			}
			return nil
		},
	)
}
