package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	pwaStandaloneUsageVersion     = "1.15.304"
	pwaStandaloneUsageDescription = "Create iot.pwa_standalone_usage - per-account PWA standalone-mode usage (#2189)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pwaStandaloneUsageVersion,
		Description: pwaStandaloneUsageDescription,
		DependsOn: []string{
			reclassifyPlannedSpontaneousInstancesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return pwaStandaloneUsageUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return pwaStandaloneUsageDown(ctx, db)
		},
	)
}

// pwaStandaloneUsageUp creates the PWA standalone-usage store (#2189).
//
// One row per (tenant, account, portal): the account has used the app in
// standalone display mode (installed to the home screen) at least once,
// with last_seen_at advancing on every report. Deliberately NO device
// identifier — the metric counts users, not devices, so the account is the
// natural key and nothing beyond what auth already knows is stored.
//
// account_id uses a PLAIN FK to auth.accounts — accounts are cross-tenant
// (guardians especially), matching push_subscriptions (1.15.234). A guardian
// linked to two schools gets one row per school, written by the parent-portal
// report fan-out, so per-school counts stay consistent with the per-school
// guardian denominators.
//
// Retention: rows with stale last_seen_at are swept by the nightly GDPR
// cleanup job (gdpr.pwa_usage_retention_days).
func pwaStandaloneUsageUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.304: Creating iot.pwa_standalone_usage...")
	return ensurePWAStandaloneUsage(ctx, db)
}

func ensurePWAStandaloneUsage(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS iot.pwa_standalone_usage (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL,
			account_id    BIGINT NOT NULL,
			portal        TEXT NOT NULL,
			first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_pwa_standalone_usage_tenant_account_portal UNIQUE (tenant_id, account_id, portal),
			CONSTRAINT chk_pwa_standalone_usage_portal CHECK (portal IN ('staff', 'parent')),
			CONSTRAINT fk_pwa_standalone_usage_tenant
				FOREIGN KEY (tenant_id)
				REFERENCES platform.schools(id)
				ON DELETE CASCADE,
			CONSTRAINT fk_pwa_standalone_usage_account
				FOREIGN KEY (account_id)
				REFERENCES auth.accounts(id)
				ON DELETE CASCADE
		);

		-- Serves the 30-day window aggregate and the retention sweep.
		CREATE INDEX IF NOT EXISTS idx_pwa_standalone_usage_last_seen
			ON iot.pwa_standalone_usage (tenant_id, portal, last_seen_at);

		DROP TRIGGER IF EXISTS update_pwa_standalone_usage_updated_at ON iot.pwa_standalone_usage;
		CREATE TRIGGER update_pwa_standalone_usage_updated_at
		BEFORE UPDATE ON iot.pwa_standalone_usage
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		DROP POLICY IF EXISTS tenant_isolation_iot_pwa_standalone_usage ON iot.pwa_standalone_usage;
		CREATE POLICY tenant_isolation_iot_pwa_standalone_usage ON iot.pwa_standalone_usage
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON iot.pwa_standalone_usage TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE iot.pwa_standalone_usage_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating iot.pwa_standalone_usage: %w", err)
	}

	return tx.Commit()
}

func pwaStandaloneUsageDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.304: Dropping iot.pwa_standalone_usage...")
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS iot.pwa_standalone_usage;`)
	if err != nil {
		return fmt.Errorf("error dropping iot.pwa_standalone_usage: %w", err)
	}
	return nil
}
