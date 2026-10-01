package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentMessageCountScopeVersion     = "1.15.434"
	parentMessageCountScopeDescription = "Let staff choose which parent messages their own counter counts, and clear it without a read receipt (#3673)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessageCountScopeVersion,
		Description: parentMessageCountScopeDescription,
		DependsOn: []string{
			parentMessageStaffMarkedUnreadVersion, // users.parent_message_reads, team mark
			AuthAccountsVersion,                   // auth.accounts (readers)
			enrollmentRequestReadsVersion,         // latest migration at authoring time
		},
	})
	Migrations.MustRegister(parentMessageCountScopeUp, parentMessageCountScopeDown)
}

// parentMessageCountScopeUp adds two pieces of personal staff state (#3673).
//
// users.parent_message_reads gets a second, personal boundary next to the read
// cursor. "Alle als gelesen markieren" moves only this boundary: it clears the
// person's own unread numbers, but it is not a read, so the parent-facing
// "Von der OGS gelesen" receipt, which follows the read cursor, stays as it
// was. Both columns are NULL together until the first clear.
//
// users.parent_message_count_preferences stores which conversations a staff
// member's own counter counts. One row per (school, account); no row means
// "all", the behaviour before this migration, so the table starts empty.
// account_id uses a plain FK like users.notification_preferences: accounts are
// cross-tenant.
func parentMessageCountScopeUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.parent_message_reads
			ADD COLUMN IF NOT EXISTS cleared_up_to_at TIMESTAMPTZ,
			ADD COLUMN IF NOT EXISTS cleared_up_to_message_id BIGINT;

		ALTER TABLE users.parent_message_reads
			DROP CONSTRAINT IF EXISTS chk_parent_message_reads_cleared_pair,
			ADD CONSTRAINT chk_parent_message_reads_cleared_pair CHECK (
				(cleared_up_to_at IS NULL) = (cleared_up_to_message_id IS NULL)
			);

		CREATE TABLE IF NOT EXISTS users.parent_message_count_preferences (
			tenant_id   BIGINT      NOT NULL,
			account_id  BIGINT      NOT NULL,
			count_scope TEXT        NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT pk_parent_message_count_preferences PRIMARY KEY (tenant_id, account_id),
			CONSTRAINT chk_parent_message_count_preferences_scope
				CHECK (count_scope IN ('all', 'own_groups', 'none')),
			CONSTRAINT fk_parent_message_count_preferences_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_parent_message_count_preferences_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_parent_message_count_preferences_account
			ON users.parent_message_count_preferences (account_id);

		DROP TRIGGER IF EXISTS update_parent_message_count_preferences_updated_at
			ON users.parent_message_count_preferences;
		CREATE TRIGGER update_parent_message_count_preferences_updated_at
		BEFORE UPDATE ON users.parent_message_count_preferences
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.parent_message_count_preferences ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.parent_message_count_preferences FORCE  ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_parent_message_count_preferences
			ON users.parent_message_count_preferences;
		CREATE POLICY tenant_isolation_users_parent_message_count_preferences
			ON users.parent_message_count_preferences
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.parent_message_count_preferences TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("add parent-message count scope: %w", err)
	}
	return tx.Commit()
}

func parentMessageCountScopeDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS users.parent_message_count_preferences;
		ALTER TABLE users.parent_message_reads
			DROP CONSTRAINT IF EXISTS chk_parent_message_reads_cleared_pair,
			DROP COLUMN IF EXISTS cleared_up_to_message_id,
			DROP COLUMN IF EXISTS cleared_up_to_at;
	`)
	if err != nil {
		return fmt.Errorf("remove parent-message count scope: %w", err)
	}
	return nil
}
