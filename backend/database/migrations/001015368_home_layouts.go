package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
)

const (
	homeLayoutsVersion     = "1.15.368"
	homeLayoutsDescription = "Create config.home_layouts and config.home_block_policies - personal start page composition"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     homeLayoutsVersion,
		Description: homeLayoutsDescription,
		DependsOn: []string{
			spontaneousActivityInstanceUniquenessVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return homeLayoutsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return homeLayoutsDown(ctx, db)
		},
	)
}

// homeLayoutsUp creates the two stores behind "Startseite anpassen" (#2875).
//
// config.home_layouts holds one row per (tenant, account): the person's own
// deviations from the recommended start page. config.home_block_policies holds
// one row per tenant: what the school prescribes for everybody.
//
// Both store a jsonb MAP rather than a row per block. The map is always read
// and written as a whole — the start page needs every entry at once and a
// partial write has no meaning — so a single row keeps the write atomic and the
// read a single-key lookup. The price is no per-block audit trail, which the
// setting_audit table would have given us; neither store is a permission
// boundary (see below), so that trade is worth the simplicity.
//
// The MAP STORES ONLY DEVIATIONS. A block that is absent falls back to the
// default its role and the school's policy give it. That is what lets a start
// page block added in a later release reach existing accounts in its intended
// default state instead of silently vanishing for everyone who ever opened the
// customize dialog.
//
// block_key values are deliberately NOT constrained by a CHECK and not
// validated against a catalogue in the database. The catalogue of start page
// blocks lives in the frontend, which is the only layer that knows a block's
// label, its permission and the operating modes it makes sense in; the API
// validates the SHAPE of a key (see models/config/home_layout.go) and nothing
// more. A key nobody renders any more is inert. This keeps adding a block a
// frontend-only change rather than a lockstep deploy.
//
// NEITHER TABLE IS A PERMISSION BOUNDARY. Showing a block cannot grant access
// to anything: every endpoint a block reads from enforces its own permissions,
// and the frontend only offers blocks the account may see. A forged key in
// this table therefore renders nothing.
//
// account_id uses a PLAIN FK to auth.accounts, matching
// users.notification_preferences (1.15.239): accounts are cross-tenant, so the
// unique key carries tenant_id and one person keeps a separate start page per
// school.
func homeLayoutsUp(ctx context.Context, db *bun.DB) error {
	slog.Info("migration starting", "migration", homeLayoutsVersion)
	return ensureHomeLayouts(ctx, db)
}

func ensureHomeLayouts(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			slog.Warn(
				"migration rollback failed",
				"migration", homeLayoutsVersion,
				"error", rollbackErr,
			)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.home_layouts (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT      NOT NULL,
			account_id BIGINT      NOT NULL,
			overrides  JSONB       NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_home_layouts_tenant_account
				UNIQUE (tenant_id, account_id),
			CONSTRAINT fk_home_layouts_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_home_layouts_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS config.home_block_policies (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT      NOT NULL,
			policies   JSONB       NOT NULL DEFAULT '{}'::jsonb,
			updated_by BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_home_block_policies_tenant
				UNIQUE (tenant_id),
			CONSTRAINT fk_home_block_policies_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_home_block_policies_updated_by
				FOREIGN KEY (updated_by) REFERENCES auth.accounts(id) ON DELETE SET NULL
		);

		DROP TRIGGER IF EXISTS update_home_layouts_updated_at ON config.home_layouts;
		CREATE TRIGGER update_home_layouts_updated_at
		BEFORE UPDATE ON config.home_layouts
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_home_block_policies_updated_at ON config.home_block_policies;
		CREATE TRIGGER update_home_block_policies_updated_at
		BEFORE UPDATE ON config.home_block_policies
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE config.home_layouts ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.home_layouts FORCE  ROW LEVEL SECURITY;
		ALTER TABLE config.home_block_policies ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.home_block_policies FORCE  ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_config_home_layouts ON config.home_layouts;
		CREATE POLICY tenant_isolation_config_home_layouts
			ON config.home_layouts
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		DROP POLICY IF EXISTS tenant_isolation_config_home_block_policies ON config.home_block_policies;
		CREATE POLICY tenant_isolation_config_home_block_policies
			ON config.home_block_policies
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON config.home_layouts TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.home_layouts_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON config.home_block_policies TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.home_block_policies_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating home layout tables: %w", err)
	}

	return tx.Commit()
}

// homeLayoutsDown drops both tables. The rows are a display preference the
// person can re-enter in a dialog, and nothing else references them.
func homeLayoutsDown(ctx context.Context, db *bun.DB) error {
	slog.Info("migration rollback starting", "migration", homeLayoutsVersion)
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS config.home_layouts;
		DROP TABLE IF EXISTS config.home_block_policies;
	`)
	if err != nil {
		return fmt.Errorf("error dropping home layout tables: %w", err)
	}
	return nil
}
