package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	mealCaterersVersion     = "1.15.467"
	mealCaterersDescription = "Caterer: Betrieb, Konten, Einladung durch die Schule und Belieferung einer Schule (#3873, ADR 0065)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     mealCaterersVersion,
		Description: mealCaterersDescription,
		DependsOn:   []string{mealRegistrationLogVersion},
	})
	Migrations.MustRegister(mealCaterersUp, mealCaterersDown)
}

// mealCaterersUp adds the caterer (ADR 0065). The caterer, its accounts and a
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
				'Caterer-Betrieb, der eine oder mehrere Schulen mit Mittagessen beliefert (#3873, ADR 0065).';

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
			-- Caterers are no school's data: only the administrative role of
			-- the login and acceptance flows reaches them.
			REVOKE ALL ON auth.caterers, auth.caterer_accounts FROM phoenix_tenant;

			ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_portal_scope;
			ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_portal_scope
				CHECK (portal_scope IN ('tenant', 'org', 'parent', 'school', 'caterer', 'unknown'));
		`); err != nil {
			return fmt.Errorf("create caterer tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "auth.caterer_invitations", "schedule.meal_caterer_supplies"); err != nil {
			return fmt.Errorf("provision caterer tenant isolation: %w", err)
		}
		return nil
	})
}

func mealCaterersDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		DELETE FROM auth.tokens WHERE portal_scope = 'caterer';
		ALTER TABLE auth.tokens DROP CONSTRAINT IF EXISTS chk_tokens_portal_scope;
		ALTER TABLE auth.tokens ADD CONSTRAINT chk_tokens_portal_scope
			CHECK (portal_scope IN ('tenant', 'org', 'parent', 'school', 'unknown'));
		DROP TABLE IF EXISTS schedule.meal_caterer_supplies;
		DROP TABLE IF EXISTS auth.caterer_invitations;
		DROP TABLE IF EXISTS auth.caterer_accounts;
		DROP TABLE IF EXISTS auth.caterers;
	`); err != nil {
		return fmt.Errorf("drop caterer tables: %w", err)
	}
	return nil
}
