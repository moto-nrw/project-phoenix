package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealCaterersVersion     = "1.15.477"
	mealCaterersDescription = "Caterer: Betrieb, Konten, Einladung durch die Schule und Belieferung einer Schule (#3873, ADR 0077)"
)

// accountIsUnusedSQL restates auth.account_is_unused of 1.15.464 (#4043);
// catererAccountsInUse adds that a caterer account is in use while its
// caterer supplies a school or an invitation for its address is open, so the
// hourly erasure never takes a working caterer's login.
const accountIsUnusedSQL = `
		CREATE OR REPLACE FUNCTION auth.account_is_unused(target_id BIGINT, target_email TEXT)
		RETURNS BOOLEAN
		LANGUAGE sql
		STABLE
		AS $$
			SELECT NOT EXISTS (
				SELECT 1 FROM auth.account_tenants AS membership
				WHERE membership.account_id = target_id
					AND (
						membership.status = 'pending'
						OR (
							membership.status = 'active'
							AND NOT EXISTS (
								SELECT 1 FROM auth.account_roles AS grant_row
								WHERE grant_row.account_id = membership.account_id
									AND grant_row.tenant_id = membership.tenant_id
							)
						)
						OR (
							membership.status = 'active'
							AND EXISTS (
								SELECT 1 FROM auth.account_roles AS grant_row
								JOIN auth.roles AS granted ON granted.id = grant_row.role_id
								WHERE grant_row.account_id = membership.account_id
									AND grant_row.tenant_id = membership.tenant_id
									AND LOWER(granted.name) <> 'guardian'
							)
						)
						OR (
							membership.status = 'active'
							AND EXISTS (
								SELECT 1 FROM auth.account_permissions AS permission
								WHERE permission.account_id = membership.account_id
									AND permission.tenant_id = membership.tenant_id
									AND permission.granted
							)
						)
					)
			)
			AND NOT EXISTS (SELECT 1 FROM users.persons AS person WHERE person.account_id = target_id)
			AND NOT EXISTS (SELECT 1 FROM users.guardian_profiles AS guardian WHERE guardian.account_id = target_id)
			AND NOT EXISTS (SELECT 1 FROM enrollment.requests AS request WHERE request.guardian_account_id = target_id)
			AND NOT EXISTS (
				SELECT 1 FROM auth.invitation_tokens AS invitation
				WHERE LOWER(invitation.email) = LOWER(target_email)
					AND invitation.used_at IS NULL
					AND invitation.expires_at > statement_timestamp()
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.guardian_invitations AS invitation
				JOIN users.guardian_profiles AS invited ON invited.id = invitation.guardian_profile_id
				WHERE LOWER(invited.email) = LOWER(target_email)
					AND invitation.accepted_at IS NULL
					AND invitation.expires_at > statement_timestamp()
					AND invitation.approval_status <> 'rejected'
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.demo_accesses AS demo
				WHERE demo.account_id = target_id OR demo.parent_account_id = target_id
			)`

const catererAccountsInUse = `
			AND NOT EXISTS (
				SELECT 1 FROM auth.caterer_accounts AS caterer_account
				JOIN auth.caterer_schools AS school
					ON school.caterer_id = caterer_account.caterer_id AND school.ended_at IS NULL
				WHERE caterer_account.account_id = target_id
			)
			AND NOT EXISTS (
				SELECT 1 FROM auth.caterer_invitations AS invitation
				WHERE LOWER(invitation.email) = LOWER(target_email)
					AND invitation.accepted_at IS NULL
					AND invitation.revoked_at IS NULL
					AND invitation.expires_at > statement_timestamp()
			)`

const accountIsUnusedEnd = `;
		$$;
		REVOKE ALL ON FUNCTION auth.account_is_unused(BIGINT, TEXT) FROM PUBLIC;
`

// catererAccountReferences fences the new account references against the
// erasure, as 1.15.464 does for every other one.
const catererAccountReferences = `
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE ON auth.caterer_accounts
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference('account_id');
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE OF created_by_account_id, accepted_account_id ON auth.caterer_invitations
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference('created_by_account_id', 'accepted_account_id');
		CREATE TRIGGER lock_live_account_reference
			BEFORE INSERT OR UPDATE OF started_by_account_id, ended_by_account_id ON schedule.meal_caterer_supplies
			FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference('started_by_account_id', 'ended_by_account_id');
`

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealCaterersVersion,
		Description: mealCaterersDescription,
		DependsOn:   []string{mealRegistrationLogVersion},
	})
	Migrations.MustRegister(mealCaterersUp, mealCaterersDown)
}

// mealCaterersUp adds the caterer (ADR 0077). auth.caterer_schools names
// the schools a caterer supplies, so the login and the caterer portal find
// them through the global identity read and open each school on its own;
// nothing searches across schools (#4088). The caterer, its accounts and a
// school's invitation are identity data; the supply of a school is the
// school's meal plan data under its RLS. A caterer reaches a school only
// through an active supply, and a school has at most one. An invitation keeps
// only the hash of its link token, so a database copy opens no account.
func mealCaterersUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			CREATE TABLE auth.caterers (
				id BIGSERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT chk_caterer_name CHECK (length(trim(name)) BETWEEN 1 AND 200)
			);
			COMMENT ON TABLE auth.caterers IS
				'Caterer-Betrieb, der eine oder mehrere Schulen mit Mittagessen beliefert (#3873, ADR 0077).';

			CREATE TABLE auth.caterer_accounts (
				account_id BIGINT PRIMARY KEY REFERENCES auth.accounts(id) ON DELETE CASCADE,
				caterer_id BIGINT NOT NULL REFERENCES auth.caterers(id) ON DELETE CASCADE,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			CREATE INDEX idx_caterer_accounts_caterer ON auth.caterer_accounts (caterer_id);
			COMMENT ON TABLE auth.caterer_accounts IS
				'Anmeldekonto eines Caterer-Betriebs; ein Konto gehört zu genau einem Betrieb.';

			CREATE TABLE auth.caterer_invitations (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				token_hash TEXT NOT NULL UNIQUE,
				email TEXT NOT NULL,
				caterer_name TEXT NOT NULL,
				delivery_address TEXT NOT NULL DEFAULT '',
				created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				expires_at TIMESTAMPTZ NOT NULL,
				accepted_at TIMESTAMPTZ,
				accepted_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				revoked_at TIMESTAMPTZ,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT chk_caterer_invitation_email CHECK (position('@' IN email) > 1),
				CONSTRAINT chk_caterer_invitation_name CHECK (length(trim(caterer_name)) BETWEEN 1 AND 200)
			);
			CREATE INDEX idx_caterer_invitations_tenant ON auth.caterer_invitations (tenant_id, created_at);
			COMMENT ON TABLE auth.caterer_invitations IS
				'Einladung einer Schule an ihren Caterer; gespeichert ist nur der Hash des Links.';

			CREATE TABLE auth.caterer_schools (
				caterer_id BIGINT NOT NULL REFERENCES auth.caterers(id) ON DELETE CASCADE,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				ended_at TIMESTAMPTZ,
				PRIMARY KEY (caterer_id, tenant_id, started_at),
				CONSTRAINT chk_caterer_school_period CHECK (ended_at IS NULL OR ended_at >= started_at)
			);
			CREATE UNIQUE INDEX uq_caterer_school_active ON auth.caterer_schools (tenant_id) WHERE ended_at IS NULL;
			CREATE INDEX idx_caterer_schools_caterer ON auth.caterer_schools (caterer_id) WHERE ended_at IS NULL;
			COMMENT ON TABLE auth.caterer_schools IS
				'Welche Schulen ein Caterer beliefert. Login und Caterer-Portal finden darüber die Schulen, ohne über alle Schulen zu suchen (#4088).';

			CREATE TABLE schedule.meal_caterer_supplies (
				id BIGSERIAL PRIMARY KEY,
				tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				caterer_id BIGINT NOT NULL REFERENCES auth.caterers(id) ON DELETE CASCADE,
				delivery_address TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				started_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				ended_at TIMESTAMPTZ,
				ended_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				CONSTRAINT chk_meal_caterer_supply_period CHECK (ended_at IS NULL OR ended_at >= started_at)
			);
			CREATE UNIQUE INDEX uq_meal_caterer_supply_active ON schedule.meal_caterer_supplies (tenant_id) WHERE ended_at IS NULL;
			CREATE INDEX idx_meal_caterer_supplies_caterer ON schedule.meal_caterer_supplies (caterer_id) WHERE ended_at IS NULL;
			COMMENT ON TABLE schedule.meal_caterer_supplies IS
				'Belieferung einer Schule durch einen Caterer; endet die Belieferung, sieht der Caterer die Schule nicht mehr.';

			GRANT SELECT, INSERT, UPDATE ON schedule.meal_caterer_supplies TO phoenix_tenant;
			REVOKE DELETE, TRUNCATE ON schedule.meal_caterer_supplies FROM phoenix_tenant;
			GRANT USAGE ON SEQUENCE schedule.meal_caterer_supplies_id_seq TO phoenix_tenant;
			GRANT SELECT, INSERT, UPDATE ON auth.caterer_invitations TO phoenix_tenant;
			REVOKE DELETE, TRUNCATE ON auth.caterer_invitations FROM phoenix_tenant;
			GRANT USAGE ON SEQUENCE auth.caterer_invitations_id_seq TO phoenix_tenant;
			-- The acceptance runs in the invited school's transaction: it
			-- creates the caterer, its account link and the school's link.
			-- Nothing there may be rewritten or deleted by a school.
			GRANT SELECT, INSERT ON auth.caterers, auth.caterer_accounts TO phoenix_tenant;
			REVOKE UPDATE, DELETE, TRUNCATE ON auth.caterers, auth.caterer_accounts FROM phoenix_tenant;
			GRANT USAGE ON SEQUENCE auth.caterers_id_seq TO phoenix_tenant;
			GRANT SELECT, INSERT, UPDATE ON auth.caterer_schools TO phoenix_tenant;
			REVOKE DELETE, TRUNCATE ON auth.caterer_schools FROM phoenix_tenant;

			ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_portal_scope;
			ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_portal_scope
				CHECK (portal_scope IN ('tenant', 'org', 'parent', 'school', 'caterer', 'unknown'));
		`); err != nil {
			return fmt.Errorf("create caterer tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "auth.caterer_invitations", "auth.caterer_schools", "schedule.meal_caterer_supplies"); err != nil {
			return fmt.Errorf("provision caterer tenant isolation: %w", err)
		}
		if _, err := tx.ExecContext(ctx, catererAccountReferences+accountIsUnusedSQL+catererAccountsInUse+accountIsUnusedEnd); err != nil {
			return fmt.Errorf("keep caterer accounts from the account erasure: %w", err)
		}
		return nil
	})
}

func mealCaterersDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, accountIsUnusedSQL+accountIsUnusedEnd); err != nil {
		return fmt.Errorf("restore the account erasure without caterers: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		DELETE FROM auth.tokens WHERE portal_scope = 'caterer';
		ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_portal_scope;
		ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_portal_scope
			CHECK (portal_scope IN ('tenant', 'org', 'parent', 'school', 'unknown'));
		DROP TABLE IF EXISTS schedule.meal_caterer_supplies;
		DROP TABLE IF EXISTS auth.caterer_schools;
		DROP TABLE IF EXISTS auth.caterer_invitations;
		DROP TABLE IF EXISTS auth.caterer_accounts;
		DROP TABLE IF EXISTS auth.caterers;
	`); err != nil {
		return fmt.Errorf("drop caterer tables: %w", err)
	}
	return nil
}
