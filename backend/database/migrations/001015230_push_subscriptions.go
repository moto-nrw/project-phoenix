package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	pushSubscriptionsVersion     = "1.15.230"
	pushSubscriptionsDescription = "Create iot.push_subscriptions - Web Push device subscriptions (#2003)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pushSubscriptionsVersion,
		Description: pushSubscriptionsDescription,
		DependsOn: []string{
			timeTrackingDeletionsAuditVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return pushSubscriptionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return pushSubscriptionsDown(ctx, db)
		},
	)
}

// pushSubscriptionsUp creates the Web Push subscription store (#2003).
//
// One row per (tenant, browser endpoint): a device that registered for push.
// account_id uses a PLAIN FK to auth.accounts — accounts are cross-tenant
// (guardians especially), so no composite tenant FK exists for them; this
// matches parent_announcements (1.15.161) and excused_absence_requests
// (1.15.177). A guardian linked to two schools gets one row per school for
// the same endpoint, which is why the unique key is (tenant_id, endpoint)
// and NOT the endpoint alone — a global unique would let one tenant's
// registration silently steal another's and would fight RLS on cleanup.
//
// portal records through which portal the device registered ('staff' or
// 'parent'): tenant/admin-scoped notifications must never reach guardian
// devices and vice versa. The two portals are different origins, so one
// browser produces distinct endpoints per portal — no collision on the
// unique key.
//
// p256dh/auth are the client-side encryption keys of the Web Push standard
// (public values, not secrets). The payload GDPR contract lives in
// services/notifications: Title/Body/DeepLink only, never child data.
func pushSubscriptionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.230: Creating iot.push_subscriptions...")

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
		CREATE TABLE IF NOT EXISTS iot.push_subscriptions (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT NOT NULL,
			account_id BIGINT NOT NULL,
			portal     TEXT NOT NULL,
			endpoint   TEXT NOT NULL,
			p256dh     TEXT NOT NULL,
			auth       TEXT NOT NULL,
			user_agent TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_push_subscriptions_tenant_endpoint UNIQUE (tenant_id, endpoint),
			CONSTRAINT chk_push_subscriptions_portal CHECK (portal IN ('staff', 'parent')),
			CONSTRAINT fk_push_subscriptions_tenant
				FOREIGN KEY (tenant_id)
				REFERENCES platform.schools(id)
				ON DELETE CASCADE,
			CONSTRAINT fk_push_subscriptions_account
				FOREIGN KEY (account_id)
				REFERENCES auth.accounts(id)
				ON DELETE CASCADE
		);

		-- Audience fan-out and per-account listing.
		CREATE INDEX IF NOT EXISTS idx_push_subscriptions_account
			ON iot.push_subscriptions (tenant_id, account_id);

		DROP TRIGGER IF EXISTS update_push_subscriptions_updated_at ON iot.push_subscriptions;
		CREATE TRIGGER update_push_subscriptions_updated_at
		BEFORE UPDATE ON iot.push_subscriptions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE iot.push_subscriptions ENABLE ROW LEVEL SECURITY;
		ALTER TABLE iot.push_subscriptions FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_iot_push_subscriptions ON iot.push_subscriptions;
		CREATE POLICY tenant_isolation_iot_push_subscriptions ON iot.push_subscriptions
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON iot.push_subscriptions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE iot.push_subscriptions_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating iot.push_subscriptions: %w", err)
	}

	return tx.Commit()
}

func pushSubscriptionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.230: Dropping iot.push_subscriptions...")

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
		DROP TRIGGER IF EXISTS update_push_subscriptions_updated_at ON iot.push_subscriptions;
		DROP TABLE IF EXISTS iot.push_subscriptions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping iot.push_subscriptions: %w", err)
	}
	return tx.Commit()
}
