package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	notificationPreferencesVersion     = "1.15.239"
	notificationPreferencesDescription = "Create users.notification_preferences - per-account notification opt-in"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     notificationPreferencesVersion,
		Description: notificationPreferencesDescription,
		DependsOn: []string{
			gradeTransitionHistoryRFIDTagVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return notificationPreferencesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return notificationPreferencesDown(ctx, db)
		},
	)
}

// notificationPreferencesUp creates the per-account notification opt-in store.
//
// One row per (tenant, account, notification type). Notifications are delivered
// only where a person has explicitly agreed to that type, so the absence of a
// row means "off" and the whole table starts empty.
//
// account_id uses a PLAIN FK to auth.accounts, matching iot.push_subscriptions
// (1.15.234): accounts are cross-tenant, guardians especially, so there is no
// composite tenant FK for them. A guardian at two schools decides per school,
// which is why the unique key carries tenant_id.
//
// notification_type is deliberately NOT constrained by a CHECK. The catalogue
// of types lives in Go (services/notifications/types.go) and is validated by the
// API; a CHECK would make every new type cost a migration and turn a rollback
// into a data problem. An unknown type in this table is inert — nothing reads it.
//
// A row with enabled = false is meaningfully different from no row at all: it
// records a deliberate opt-out, which a future change of defaults must not
// silently overwrite.
func notificationPreferencesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.239: Creating users.notification_preferences...")
	return ensureNotificationPreferences(ctx, db)
}

func ensureNotificationPreferences(ctx context.Context, db *bun.DB) error {
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
		CREATE TABLE IF NOT EXISTS users.notification_preferences (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT      NOT NULL,
			account_id        BIGINT      NOT NULL,
			notification_type TEXT        NOT NULL,
			enabled           BOOLEAN     NOT NULL DEFAULT FALSE,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_notification_preferences
				UNIQUE (tenant_id, account_id, notification_type),
			CONSTRAINT fk_notification_preferences_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_notification_preferences_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		);

		-- The producers' hot question is "who in this school agreed to type X".
		-- Partial on enabled because that query never wants the opt-out rows.
		CREATE INDEX IF NOT EXISTS idx_notification_preferences_type
			ON users.notification_preferences (tenant_id, notification_type)
			WHERE enabled;

		DROP TRIGGER IF EXISTS update_notification_preferences_updated_at
			ON users.notification_preferences;
		CREATE TRIGGER update_notification_preferences_updated_at
		BEFORE UPDATE ON users.notification_preferences
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.notification_preferences ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.notification_preferences FORCE  ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_notification_preferences
			ON users.notification_preferences;
		CREATE POLICY tenant_isolation_users_notification_preferences
			ON users.notification_preferences
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.notification_preferences TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.notification_preferences_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.notification_preferences: %w", err)
	}

	return tx.Commit()
}

// notificationPreferencesDown drops the table. Unlike 1.15.234's deliberate
// no-op, the data here is a user's own choice that they can re-enter, and
// nothing else references the rows.
func notificationPreferencesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.239: Dropping users.notification_preferences...")
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS users.notification_preferences;`)
	if err != nil {
		return fmt.Errorf("error dropping users.notification_preferences: %w", err)
	}
	return nil
}
