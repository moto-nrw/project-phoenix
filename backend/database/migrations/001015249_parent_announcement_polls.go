package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	parentAnnouncementPollsVersion     = "1.15.249"
	parentAnnouncementPollsDescription = "Add poll (Umfrage) response options and per-child responses to parent announcements"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentAnnouncementPollsVersion,
		Description: parentAnnouncementPollsDescription,
		DependsOn: []string{
			parentAnnouncementsVersion, // the announcement tables this extends
			"1.3.5",                    // users.students (per-child responses)
			"1.0.1",                    // auth.accounts (who answered)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentAnnouncementPollsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentAnnouncementPollsDown(ctx, db)
		},
	)
}

func parentAnnouncementPollsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.249: Adding parent announcement poll columns and tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// A poll (Umfrage) is an announcement that carries answer options. Everything
	// else about it — targeting, draft/publish, expiry, e-mail opt-in, push — is
	// the announcement machinery it inherits, which is why this is columns on the
	// existing table rather than a parallel entity (#1371).
	//
	// response_type = 'none' is a plain Mitteilung (every pre-existing row).
	// response_deadline is the answer cut-off; it is deliberately separate from
	// expires_at, which controls VISIBILITY: a school wants the result of a closed
	// poll to stay readable in the feed after answering has ended.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS response_type     TEXT NOT NULL DEFAULT 'none',
			ADD COLUMN IF NOT EXISTS response_deadline TIMESTAMPTZ;
	`)
	if err != nil {
		return fmt.Errorf("error adding poll columns to users.parent_announcements: %w", err)
	}

	// Constraints are added separately (ADD CONSTRAINT has no IF NOT EXISTS) so a
	// re-run of the migration on a partially applied schema does not fail.
	_, err = tx.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_response_type'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_response_type
					CHECK (response_type IN ('none','single_choice','multi_choice'));
			END IF;
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_response_deadline'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_response_deadline
					CHECK (response_deadline IS NULL OR response_type <> 'none');
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error adding poll constraints to users.parent_announcements: %w", err)
	}

	// One answer option per row. position drives the display order and is unique
	// per announcement so two options can never render in an ambiguous order.
	// Options are insert/delete only (replaced wholesale while the announcement is
	// still a draft), so there is no updated_at column and no trigger.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_announcement_options (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id BIGINT NOT NULL REFERENCES users.parent_announcements(id) ON DELETE CASCADE,
			label           TEXT NOT NULL,
			position        INT NOT NULL,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_parent_announcement_options_label_not_blank CHECK (length(btrim(label)) > 0),
			CONSTRAINT uq_parent_announcement_options_position UNIQUE (announcement_id, position)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_announcement_options_announcement
			ON users.parent_announcement_options (tenant_id, announcement_id, position);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_announcement_options: %w", err)
	}

	// One row per (child, chosen option). The answer is PER CHILD, not per
	// guardian account (#1371 decision): "Kommt Ihr Kind zur Murmelparty" is a
	// question about the child, and a parent with two children at the OGS answers
	// once per child. account_id records WHICH guardian answered (two guardians
	// share one child; last writer wins, the service replaces the child's row set).
	//
	// A single_choice poll keeps at most one row per child; multi_choice keeps one
	// row per selected option. The unique constraint makes a double-submit
	// idempotent rather than duplicating a vote.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_announcement_responses (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id BIGINT NOT NULL REFERENCES users.parent_announcements(id) ON DELETE CASCADE,
			option_id       BIGINT NOT NULL REFERENCES users.parent_announcement_options(id) ON DELETE CASCADE,
			student_id      BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			account_id      BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			responded_at    TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT uq_parent_announcement_responses UNIQUE (announcement_id, student_id, option_id)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_announcement_responses_announcement
			ON users.parent_announcement_responses (tenant_id, announcement_id);
		CREATE INDEX IF NOT EXISTS idx_parent_announcement_responses_student
			ON users.parent_announcement_responses (announcement_id, student_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_announcement_responses: %w", err)
	}

	// RLS — same shape as the announcement tables this extends.
	for _, table := range []string{"parent_announcement_options", "parent_announcement_responses"} {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE users.%[1]s ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.%[1]s FORCE ROW LEVEL SECURITY;

			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM pg_policies
					WHERE schemaname = 'users'
						AND tablename = '%[1]s'
						AND policyname = 'tenant_isolation_users_%[1]s'
				) THEN
					CREATE POLICY tenant_isolation_users_%[1]s ON users.%[1]s
						FOR ALL
						USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
						WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
				END IF;
			END $$;

			GRANT SELECT, INSERT, UPDATE, DELETE ON users.%[1]s TO phoenix_tenant;
		`, table))
		if err != nil {
			return fmt.Errorf("error enabling RLS on users.%s: %w", table, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SEQUENCE users.parent_announcement_options_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.parent_announcement_responses_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting sequence usage: %w", err)
	}

	return tx.Commit()
}

func parentAnnouncementPollsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.249: Dropping parent announcement poll tables and columns...")

	if _, err := db.NewRaw(`
		DROP TABLE IF EXISTS users.parent_announcement_responses CASCADE;
		DROP TABLE IF EXISTS users.parent_announcement_options CASCADE;
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_response_deadline,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_response_type,
			DROP COLUMN IF EXISTS response_deadline,
			DROP COLUMN IF EXISTS response_type;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping parent announcement poll objects: %w", err)
	}
	return nil
}
